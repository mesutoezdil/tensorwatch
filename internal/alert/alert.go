package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/mesutoezdil/tensorwatch/internal/config"
	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type Engine struct {
	rules   []config.AlertRule
	webhook string
	state   map[string]time.Time
	logger  *log.Logger
}

func New(rules []config.AlertRule, webhook string, logger *log.Logger) *Engine {
	if logger == nil {
		logger = log.Default()
	}
	return &Engine{
		rules:   rules,
		webhook: webhook,
		state:   make(map[string]time.Time),
		logger:  logger,
	}
}

func (e *Engine) Consume(ctx context.Context, src <-chan model.Snapshot) {
	for {
		select {
		case <-ctx.Done():
			return
		case s, ok := <-src:
			if !ok {
				return
			}
			e.evaluate(ctx, s)
		}
	}
}

func (e *Engine) evaluate(ctx context.Context, s model.Snapshot) {
	now := time.Now()
	for _, r := range e.rules {
		val, ok := metricValue(s, r.Metric)
		if !ok {
			continue
		}
		breached := compare(val, r.Operator, r.Value)
		key := r.Name
		if !breached {
			delete(e.state, key)
			continue
		}
		first, seen := e.state[key]
		if !seen {
			e.state[key] = now
			first = now
		}
		if now.Sub(first) >= r.Sustain.Duration {
			e.fire(ctx, r, val)
			e.state[key] = now.Add(time.Hour)
		}
	}
}

func (e *Engine) fire(ctx context.Context, r config.AlertRule, value float64) {
	msg := fmt.Sprintf("[alert] %s: %s %s %.2f (current %.2f)", r.Name, r.Metric, r.Operator, r.Value, value)
	e.logger.Println(msg)
	if e.webhook == "" {
		return
	}
	payload := map[string]any{
		"alert":   r.Name,
		"metric":  r.Metric,
		"value":   value,
		"limit":   r.Value,
		"op":      r.Operator,
		"time":    time.Now().UTC().Format(time.RFC3339),
		"message": msg,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.webhook, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		e.logger.Printf("alert webhook error: %v", err)
		return
	}
	resp.Body.Close()
}

func metricValue(s model.Snapshot, name string) (float64, bool) {
	switch name {
	case "cpu.overall":
		return s.CPU.UsageOverall, true
	case "cpu.temp":
		return s.CPU.TempCelsius, true
	case "mem.used_pct":
		return s.Memory.UsedPct, true
	case "load.1":
		return s.Host.Load1, true
	case "gpu.util":
		var max float64
		for _, g := range s.GPUs {
			if g.UtilGPU > max {
				max = g.UtilGPU
			}
		}
		return max, len(s.GPUs) > 0
	case "gpu.temp":
		var max float64
		for _, g := range s.GPUs {
			if g.TempCelsius > max {
				max = g.TempCelsius
			}
		}
		return max, len(s.GPUs) > 0
	case "gpu.mem_used_pct":
		var max float64
		for _, g := range s.GPUs {
			if g.MemTotal == 0 {
				continue
			}
			pct := float64(g.MemUsed) / float64(g.MemTotal) * 100
			if pct > max {
				max = pct
			}
		}
		return max, len(s.GPUs) > 0
	}
	return 0, false
}

func compare(val float64, op string, threshold float64) bool {
	switch op {
	case ">":
		return val > threshold
	case ">=":
		return val >= threshold
	case "<":
		return val < threshold
	case "<=":
		return val <= threshold
	case "==":
		return val == threshold
	}
	return false
}
