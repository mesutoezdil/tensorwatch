package host

import (
	"context"
	"runtime"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type Collector struct{}

func New() *Collector { return &Collector{} }

func (c *Collector) Name() string { return "host" }

func (c *Collector) Collect(ctx context.Context, into *model.Snapshot) error {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return err
	}
	into.Host.Hostname = info.Hostname
	into.Host.OS = info.Platform + " " + info.PlatformVersion
	into.Host.Kernel = info.KernelVersion
	into.Host.Arch = runtime.GOARCH
	into.Host.UptimeSec = info.Uptime
	into.Host.BootTimeUnix = int64(info.BootTime)

	if l, err := load.AvgWithContext(ctx); err == nil {
		into.Host.Load1 = l.Load1
		into.Host.Load5 = l.Load5
		into.Host.Load15 = l.Load15
	}
	return nil
}

func (c *Collector) Close() error { return nil }
