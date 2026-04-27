package processes

import (
	"context"
	"sort"
	"sync"

	"github.com/shirou/gopsutil/v4/process"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type Collector struct {
	limit int

	mu      sync.Mutex
	prevCPU map[int32]float64
}

func New(limit int) *Collector {
	if limit <= 0 {
		limit = 8
	}
	return &Collector{limit: limit, prevCPU: make(map[int32]float64)}
}

func (c *Collector) Name() string { return "processes" }

func (c *Collector) Close() error { return nil }

func (c *Collector) Collect(ctx context.Context, into *model.Snapshot) error {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]model.Process, 0, len(procs))
	current := make(map[int32]float64, len(procs))
	for _, p := range procs {
		cpu, err := p.CPUPercentWithContext(ctx)
		if err != nil {
			continue
		}
		current[p.Pid] = cpu
		mem, _ := p.MemoryPercentWithContext(ctx)
		mi, _ := p.MemoryInfoWithContext(ctx)
		var rss uint64
		if mi != nil {
			rss = mi.RSS
		}
		name, _ := p.NameWithContext(ctx)
		user, _ := p.UsernameWithContext(ctx)
		out = append(out, model.Process{
			PID:     p.Pid,
			User:    user,
			Command: name,
			CPUPct:  cpu,
			MemPct:  float64(mem),
			RSS:     rss,
		})
	}
	c.prevCPU = current

	sort.Slice(out, func(i, j int) bool { return out[i].CPUPct > out[j].CPUPct })
	if len(out) > c.limit {
		out = out[:c.limit]
	}
	into.Processes = out
	return nil
}
