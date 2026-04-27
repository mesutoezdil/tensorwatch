package cpu

import (
	"context"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/sensors"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type Collector struct {
	model string
}

func New() *Collector { return &Collector{} }

func (c *Collector) Name() string { return "cpu" }

func (c *Collector) Collect(ctx context.Context, into *model.Snapshot) error {
	if c.model == "" {
		if infos, err := cpu.InfoWithContext(ctx); err == nil && len(infos) > 0 {
			c.model = infos[0].ModelName
		}
	}
	logical, _ := cpu.CountsWithContext(ctx, true)
	physical, _ := cpu.CountsWithContext(ctx, false)
	into.CPU.LogicalCores = logical
	into.CPU.PhysicalCores = physical
	into.CPU.ModelName = c.model

	per, err := cpu.PercentWithContext(ctx, 0, true)
	if err != nil {
		return err
	}
	into.CPU.UsagePerCore = per
	if avg, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(avg) > 0 {
		into.CPU.UsageOverall = avg[0]
	}

	if infos, err := cpu.InfoWithContext(ctx); err == nil && len(infos) > 0 {
		into.CPU.FreqMHz = infos[0].Mhz
	}

	if temps, err := sensors.TemperaturesWithContext(ctx); err == nil {
		into.CPU.TempCelsius = pickCPUTemp(temps)
	}
	return nil
}

func (c *Collector) Close() error { return nil }

func pickCPUTemp(temps []sensors.TemperatureStat) float64 {
	var best float64
	for _, t := range temps {
		key := t.SensorKey
		if key == "" || t.Temperature <= 0 {
			continue
		}
		if isCPUSensor(key) && t.Temperature > best {
			best = t.Temperature
		}
	}
	return best
}

func isCPUSensor(key string) bool {
	for _, hint := range []string{"coretemp", "cpu", "k10temp", "package", "tdie", "tctl"} {
		if containsFold(key, hint) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	ls, lsub := toLower(s), toLower(sub)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
