package memory

import (
	"context"

	"github.com/shirou/gopsutil/v4/mem"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type Collector struct{}

func New() *Collector { return &Collector{} }

func (c *Collector) Name() string { return "memory" }

func (c *Collector) Collect(ctx context.Context, into *model.Snapshot) error {
	v, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return err
	}
	into.Memory.TotalBytes = v.Total
	into.Memory.AvailableBytes = v.Available
	into.Memory.UsedBytes = v.Used
	into.Memory.UsedPct = v.UsedPercent
	into.Memory.BufCacheBytes = v.Buffers + v.Cached

	if s, err := mem.SwapMemoryWithContext(ctx); err == nil {
		into.Memory.SwapTotal = s.Total
		into.Memory.SwapUsed = s.Used
	}
	return nil
}

func (c *Collector) Close() error { return nil }
