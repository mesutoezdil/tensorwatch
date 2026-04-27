package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Interval     Duration       `yaml:"interval"`
	TUI          TUI            `yaml:"tui"`
	HTTP         HTTP           `yaml:"http"`
	CSV          CSV            `yaml:"csv"`
	Alerts       []AlertRule    `yaml:"alerts"`
	AlertWebhook string         `yaml:"alert_webhook"`
}

type TUI struct {
	Enabled bool `yaml:"enabled"`
}

type HTTP struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
	Token   string `yaml:"token"`
}

type CSV struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type AlertRule struct {
	Name     string  `yaml:"name"`
	Metric   string  `yaml:"metric"`
	Operator string  `yaml:"operator"`
	Value    float64 `yaml:"value"`
	Sustain  Duration `yaml:"sustain"`
}

type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("duration %q: %w", raw, err)
	}
	d.Duration = parsed
	return nil
}

func Default() Config {
	return Config{
		Interval: Duration{Duration: time.Second},
		TUI:      TUI{Enabled: true},
		HTTP:     HTTP{Enabled: false, Addr: ":9123"},
		CSV:      CSV{Enabled: false, Path: "tensorwatch.csv"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
