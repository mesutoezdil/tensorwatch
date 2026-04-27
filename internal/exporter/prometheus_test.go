package exporter

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

func TestPrometheusRenderEmpty(t *testing.T) {
	p := NewPrometheus()
	out := string(p.Render())
	if !strings.Contains(out, "no data yet") {
		t.Fatalf("expected placeholder, got %q", out)
	}
}

func TestPrometheusRendersSnapshot(t *testing.T) {
	p := NewPrometheus()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan model.Snapshot, 1)
	ch <- model.Snapshot{
		Taken: time.Now(),
		Host:  model.Host{UptimeSec: 42, Load1: 1.5},
		CPU: model.CPU{
			UsageOverall: 12.5,
			UsagePerCore: []float64{10, 20},
		},
		Memory: model.Memory{TotalBytes: 1024, UsedBytes: 256, UsedPct: 25},
		GPUs: []model.GPU{{
			Index: 0, Vendor: "nvidia", Name: "TestGPU",
			UtilGPU: 80, MemTotal: 1000, MemUsed: 500, TempCelsius: 65, PowerWatts: 120,
		}},
	}
	close(ch)
	p.Consume(ctx, ch)
	cancel()

	out := string(p.Render())
	for _, want := range []string{
		"tw_cpu_usage_overall_percent",
		"tw_gpu_utilization_percent",
		"tw_memory_used_bytes",
		`gpu="0"`,
		`vendor="nvidia"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q\n---\n%s", want, out)
		}
	}
}
