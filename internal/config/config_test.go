package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefault(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval.Duration != time.Second {
		t.Errorf("default interval = %v, want 1s", cfg.Interval.Duration)
	}
	if !cfg.TUI.Enabled {
		t.Error("expected TUI enabled by default")
	}
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	body := `
interval: 250ms
http:
  enabled: true
  addr: ":8080"
alerts:
  - name: test
    metric: cpu.overall
    operator: ">"
    value: 90
    sustain: 10s
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval.Duration != 250*time.Millisecond {
		t.Errorf("interval = %v", cfg.Interval.Duration)
	}
	if !cfg.HTTP.Enabled || cfg.HTTP.Addr != ":8080" {
		t.Errorf("http config not loaded: %+v", cfg.HTTP)
	}
	if len(cfg.Alerts) != 1 || cfg.Alerts[0].Sustain.Duration != 10*time.Second {
		t.Errorf("alerts not loaded: %+v", cfg.Alerts)
	}
}
