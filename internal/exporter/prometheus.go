package exporter

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type Prometheus struct {
	mu   sync.RWMutex
	last *model.Snapshot
}

func NewPrometheus() *Prometheus { return &Prometheus{} }

func (p *Prometheus) Consume(ctx context.Context, src <-chan model.Snapshot) {
	for {
		select {
		case <-ctx.Done():
			return
		case s, ok := <-src:
			if !ok {
				return
			}
			p.mu.Lock()
			cp := s
			p.last = &cp
			p.mu.Unlock()
		}
	}
}

func (p *Prometheus) Render() []byte {
	p.mu.RLock()
	s := p.last
	p.mu.RUnlock()
	if s == nil {
		return []byte("# tensorwatch: no data yet\n")
	}

	w := newMetricsWriter()
	w.gauge("tw_uptime_seconds", "host uptime in seconds", float64(s.Host.UptimeSec), nil)
	w.gauge("tw_load_average", "system load average", s.Host.Load1, map[string]string{"window": "1m"})
	w.gauge("tw_load_average", "system load average", s.Host.Load5, map[string]string{"window": "5m"})
	w.gauge("tw_load_average", "system load average", s.Host.Load15, map[string]string{"window": "15m"})

	w.gauge("tw_cpu_usage_overall_percent", "aggregate CPU utilization", s.CPU.UsageOverall, nil)
	for i, u := range s.CPU.UsagePerCore {
		w.gauge("tw_cpu_usage_percent", "per-core CPU utilization", u, map[string]string{"core": strconv.Itoa(i)})
	}
	if s.CPU.TempCelsius > 0 {
		w.gauge("tw_cpu_temperature_celsius", "highest CPU temperature reading", s.CPU.TempCelsius, nil)
	}
	if s.CPU.FreqMHz > 0 {
		w.gauge("tw_cpu_frequency_mhz", "reported CPU frequency in MHz", s.CPU.FreqMHz, nil)
	}

	w.gauge("tw_memory_total_bytes", "total system memory", float64(s.Memory.TotalBytes), nil)
	w.gauge("tw_memory_used_bytes", "used system memory", float64(s.Memory.UsedBytes), nil)
	w.gauge("tw_memory_available_bytes", "available system memory", float64(s.Memory.AvailableBytes), nil)
	w.gauge("tw_memory_bufcache_bytes", "buffer and cache memory", float64(s.Memory.BufCacheBytes), nil)
	w.gauge("tw_swap_total_bytes", "total swap", float64(s.Memory.SwapTotal), nil)
	w.gauge("tw_swap_used_bytes", "used swap", float64(s.Memory.SwapUsed), nil)

	for _, g := range s.GPUs {
		idx := strconv.Itoa(g.Index)
		labels := map[string]string{"gpu": idx, "vendor": g.Vendor, "name": g.Name}
		w.gauge("tw_gpu_utilization_percent", "GPU compute utilization", g.UtilGPU, labels)
		w.gauge("tw_gpu_memory_utilization_percent", "GPU memory bandwidth utilization", g.UtilMemory, labels)
		w.gauge("tw_gpu_memory_total_bytes", "GPU total memory", float64(g.MemTotal), labels)
		w.gauge("tw_gpu_memory_used_bytes", "GPU used memory", float64(g.MemUsed), labels)
		w.gauge("tw_gpu_temperature_celsius", "GPU temperature", g.TempCelsius, labels)
		w.gauge("tw_gpu_power_watts", "GPU power draw", g.PowerWatts, labels)
		if g.PowerLimitW > 0 {
			w.gauge("tw_gpu_power_limit_watts", "GPU power management limit", g.PowerLimitW, labels)
		}
		if g.ClockCore > 0 {
			w.gauge("tw_gpu_clock_mhz", "GPU clock", float64(g.ClockCore), mergeLabels(labels, map[string]string{"domain": "graphics"}))
		}
		if g.ClockMem > 0 {
			w.gauge("tw_gpu_clock_mhz", "GPU clock", float64(g.ClockMem), mergeLabels(labels, map[string]string{"domain": "memory"}))
		}
		if g.FanPercent > 0 {
			w.gauge("tw_gpu_fan_percent", "GPU fan speed", g.FanPercent, labels)
		}
	}
	return w.bytes()
}

type metricsWriter struct {
	buf  []byte
	seen map[string]bool
}

func newMetricsWriter() *metricsWriter {
	return &metricsWriter{seen: make(map[string]bool)}
}

func (w *metricsWriter) gauge(name, help string, value float64, labels map[string]string) {
	if !w.seen[name] {
		w.seen[name] = true
		w.buf = append(w.buf, fmt.Sprintf("# HELP %s %s\n# TYPE %s gauge\n", name, help, name)...)
	}
	w.buf = append(w.buf, name...)
	if len(labels) > 0 {
		w.buf = append(w.buf, '{')
		first := true
		for k, v := range labels {
			if !first {
				w.buf = append(w.buf, ',')
			}
			first = false
			w.buf = append(w.buf, fmt.Sprintf("%s=%q", k, v)...)
		}
		w.buf = append(w.buf, '}')
	}
	w.buf = append(w.buf, ' ')
	w.buf = append(w.buf, strconv.FormatFloat(value, 'f', -1, 64)...)
	w.buf = append(w.buf, '\n')
}

func (w *metricsWriter) bytes() []byte { return w.buf }

func mergeLabels(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
