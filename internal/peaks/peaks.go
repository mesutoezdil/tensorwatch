package peaks

import (
	"context"
	"sync"
	"time"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type sample struct {
	at      time.Time
	cpu     float64
	gpu     float64
	mem     float64
	gpuTemp float64
	cpuTemp float64
}

type Tracker struct {
	mu      sync.RWMutex
	window  time.Duration
	samples []sample
}

func New(window time.Duration) *Tracker {
	if window < time.Second {
		window = 30 * time.Minute
	}
	return &Tracker{window: window}
}

func (t *Tracker) Window() time.Duration { return t.window }

func (t *Tracker) Observe(s model.Snapshot) {
	sm := sample{
		at:      s.Taken,
		cpu:     s.CPU.UsageOverall,
		mem:     s.Memory.UsedPct,
		cpuTemp: s.CPU.TempCelsius,
	}
	for _, g := range s.GPUs {
		if g.UtilGPU > sm.gpu {
			sm.gpu = g.UtilGPU
		}
		if g.TempCelsius > sm.gpuTemp {
			sm.gpuTemp = g.TempCelsius
		}
	}
	t.mu.Lock()
	t.samples = append(t.samples, sm)
	cutoff := s.Taken.Add(-t.window)
	idx := 0
	for idx < len(t.samples) && t.samples[idx].at.Before(cutoff) {
		idx++
	}
	if idx > 0 {
		t.samples = t.samples[idx:]
	}
	t.mu.Unlock()
}

func (t *Tracker) Decorate(s *model.Snapshot) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	p := model.Peaks{WindowSec: int(t.window / time.Second)}
	for _, sm := range t.samples {
		if sm.cpu > p.CPU {
			p.CPU = sm.cpu
		}
		if sm.gpu > p.GPU {
			p.GPU = sm.gpu
		}
		if sm.mem > p.MemPct {
			p.MemPct = sm.mem
		}
		if sm.gpuTemp > p.GPUTemp {
			p.GPUTemp = sm.gpuTemp
		}
		if sm.cpuTemp > p.CPUTemp {
			p.CPUTemp = sm.cpuTemp
		}
	}
	s.Peaks = p
}

func (t *Tracker) Consume(ctx context.Context, src <-chan model.Snapshot) {
	for {
		select {
		case <-ctx.Done():
			return
		case s, ok := <-src:
			if !ok {
				return
			}
			t.Observe(s)
		}
	}
}
