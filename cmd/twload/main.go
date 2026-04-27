package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

func main() {
	var (
		workers  = flag.Int("workers", runtime.NumCPU(), "number of CPU workers")
		duration = flag.Duration("duration", 0, "stop after this duration (0 = run until SIGINT)")
		pattern  = flag.String("pattern", "sine", "load pattern: sine | step | spike | constant")
		base     = flag.Float64("base", 50, "base load percent (0-100)")
		amp      = flag.Float64("amplitude", 50, "wave amplitude percent (sine/step/spike)")
		period   = flag.Duration("period", 30*time.Second, "wave period")
	)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `twload - synthetic CPU load generator for tensorwatch validation

Patterns:
  sine     smooth sinusoidal sweep (base ± amplitude)
  step     square wave alternating between (base) and (base + amplitude)
  spike    short bursts to (base + amplitude) every period, otherwise (base)
  constant flat load at (base)%%

Examples:
  twload                                  # 50%% sine wave on all cores, 30s period
  twload -workers 4 -pattern step         # 4 workers, square wave
  twload -base 80 -amplitude 20 -period 5s -duration 2m

`)
		flag.PrintDefaults()
	}
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *duration > 0 {
		var c context.CancelFunc
		ctx, c = context.WithTimeout(ctx, *duration)
		defer c()
	}

	gen, ok := patterns[*pattern]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown pattern %q\n", *pattern)
		os.Exit(2)
	}

	fmt.Printf("twload: %d workers, pattern=%s, base=%.0f%%, amplitude=%.0f%%, period=%s\n",
		*workers, *pattern, *base, *amp, *period)
	fmt.Println("(ctrl+c to stop)")

	var wg sync.WaitGroup
	wg.Add(*workers)
	for i := 0; i < *workers; i++ {
		go func(id int) {
			defer wg.Done()
			runWorker(ctx, gen, *base, *amp, *period, time.Duration(id)*time.Millisecond*7)
		}(i)
	}
	wg.Wait()
	fmt.Println("twload: stopped")
}

type generator func(elapsed, period time.Duration, base, amp float64) float64

var patterns = map[string]generator{
	"sine": func(t, p time.Duration, base, amp float64) float64 {
		phase := 2 * math.Pi * float64(t) / float64(p)
		return clamp(base + amp*math.Sin(phase))
	},
	"step": func(t, p time.Duration, base, amp float64) float64 {
		half := p / 2
		if (t/half)%2 == 0 {
			return clamp(base)
		}
		return clamp(base + amp)
	},
	"spike": func(t, p time.Duration, base, amp float64) float64 {
		within := t % p
		if within < p/8 {
			return clamp(base + amp)
		}
		return clamp(base)
	},
	"constant": func(_, _ time.Duration, base, _ float64) float64 {
		return clamp(base)
	},
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func runWorker(ctx context.Context, gen generator, base, amp float64, period, offset time.Duration) {
	const slice = 100 * time.Millisecond
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		elapsed := time.Since(start) + offset
		target := gen(elapsed, period, base, amp)
		busy := time.Duration(float64(slice) * target / 100)
		idle := slice - busy

		stop := time.Now().Add(busy)
		for time.Now().Before(stop) {
			_ = math.Sqrt(float64(time.Now().UnixNano()))
		}
		if idle > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(idle):
			}
		}
	}
}
