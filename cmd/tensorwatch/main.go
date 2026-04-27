package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mesutoezdil/tensorwatch/internal/alert"
	"github.com/mesutoezdil/tensorwatch/internal/collector"
	cpuc "github.com/mesutoezdil/tensorwatch/internal/collector/cpu"
	gpuc "github.com/mesutoezdil/tensorwatch/internal/collector/gpu"
	hostc "github.com/mesutoezdil/tensorwatch/internal/collector/host"
	memc "github.com/mesutoezdil/tensorwatch/internal/collector/memory"
	procc "github.com/mesutoezdil/tensorwatch/internal/collector/processes"
	"github.com/mesutoezdil/tensorwatch/internal/config"
	"github.com/mesutoezdil/tensorwatch/internal/exporter"
	"github.com/mesutoezdil/tensorwatch/internal/httpapi"
	"github.com/mesutoezdil/tensorwatch/internal/peaks"
	"github.com/mesutoezdil/tensorwatch/internal/pipeline"
	"github.com/mesutoezdil/tensorwatch/internal/tui"
)

var version = "0.2.0"

func displayVersion(v string) string {
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

func main() {
	var (
		configPath = flag.String("config", "", "path to YAML config file")
		interval   = flag.Duration("interval", 0, "sampling interval (overrides config)")
		httpAddr   = flag.String("http", "", "expose HTTP API on host:port (e.g. :9123)")
		httpToken  = flag.String("http-token", "", "bearer token for HTTP endpoints")
		csvPath    = flag.String("csv", "", "append samples to CSV file")
		headless   = flag.Bool("headless", false, "disable TUI")
		compact    = flag.Bool("compact", false, "compact TUI (no process table)")
		topN       = flag.Int("top", 8, "number of processes shown in TUI")
		peakWindow = flag.Duration("peak-window", 30*time.Minute, "rolling window for peak tracking")
		showVer    = flag.Bool("version", false, "print version and exit")
	)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `tensorwatch %s - pluggable observability agent for compute hosts

Usage:
  tensorwatch [flags]

Examples:
  tensorwatch                              # interactive TUI
  tensorwatch -interval 500ms              # half-second refresh
  tensorwatch -http :9123                  # TUI + Prometheus / JSON HTTP API
  tensorwatch -headless -http :9123        # exporter only
  tensorwatch -csv samples.csv             # append metrics to CSV
  tensorwatch -compact                     # smaller layout, no process table
  tensorwatch -config examples/config.yaml # YAML config (flags still override)

Flags:
`, version)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVer {
		fmt.Println("tensorwatch", version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if *interval > 0 {
		cfg.Interval.Duration = *interval
	}
	if *httpAddr != "" {
		cfg.HTTP.Enabled = true
		cfg.HTTP.Addr = *httpAddr
	}
	if *httpToken != "" {
		cfg.HTTP.Token = *httpToken
	}
	if envToken := os.Getenv("TENSORWATCH_TOKEN"); envToken != "" && cfg.HTTP.Token == "" {
		cfg.HTTP.Token = envToken
	}
	if *csvPath != "" {
		cfg.CSV.Enabled = true
		cfg.CSV.Path = *csvPath
	}
	if *headless {
		cfg.TUI.Enabled = false
	}
	if cfg.Interval.Duration == 0 {
		cfg.Interval.Duration = time.Second
	}

	if !cfg.TUI.Enabled && !cfg.HTTP.Enabled && !cfg.CSV.Enabled {
		fmt.Fprintln(os.Stderr, "tensorwatch: at least one output must be enabled (TUI, HTTP, or CSV)")
		os.Exit(2)
	}

	gpuCol := gpuc.New()
	collectors := collector.Set{
		hostc.New(),
		cpuc.New(),
		memc.New(),
	}
	if cfg.TUI.Enabled && !*compact {
		collectors = append(collectors, procc.New(*topN))
	}
	if gpuCol.Available() {
		collectors = append(collectors, gpuCol)
	}

	peakTracker := peaks.New(*peakWindow)
	pipe := pipeline.New(collectors, cfg.Interval.Duration, peakTracker.Decorate)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go peakTracker.Consume(ctx, pipe.Subscribe(8))
	go func() {
		if err := pipe.Run(ctx); err != nil {
			log.Printf("pipeline: %v", err)
		}
		collectors.Close()
	}()

	logger := log.New(os.Stderr, "tensorwatch ", log.LstdFlags)

	var prom *exporter.Prometheus
	if cfg.HTTP.Enabled {
		prom = exporter.NewPrometheus()
		go prom.Consume(ctx, pipe.Subscribe(8))
		srv := httpapi.New(cfg.HTTP.Addr, cfg.HTTP.Token, pipe, prom)
		go func() {
			if err := srv.Run(ctx); err != nil {
				logger.Printf("http: %v", err)
			}
		}()
		logger.Printf("HTTP API listening on %s", cfg.HTTP.Addr)
	}

	var csvSink *exporter.CSVSink
	if cfg.CSV.Enabled {
		s, err := exporter.OpenCSV(cfg.CSV.Path)
		if err != nil {
			logger.Fatalf("csv: %v", err)
		}
		csvSink = s
		go csvSink.Consume(ctx, pipe.Subscribe(64))
	}

	if len(cfg.Alerts) > 0 {
		eng := alert.New(cfg.Alerts, cfg.AlertWebhook, logger)
		go eng.Consume(ctx, pipe.Subscribe(8))
	}

	if cfg.TUI.Enabled {
		ui, err := tui.New(pipe.Subscribe(8), tui.Options{Compact: *compact, Version: displayVersion(version)})
		if err != nil {
			logger.Fatalf("tui: %v", err)
		}
		if err := ui.Run(ctx); err != nil {
			logger.Printf("tui: %v", err)
		}
		cancel()
	} else {
		<-ctx.Done()
	}

	if csvSink != nil {
		_ = csvSink.Close()
	}
}
