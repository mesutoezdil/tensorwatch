//go:build nvidia

package gpu

import (
	"context"
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type nvidiaCollector struct {
	devices []nvml.Device
	ready   bool
}

func New() Collector {
	c := &nvidiaCollector{}
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		return Noop()
	}
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		_ = nvml.Shutdown()
		return Noop()
	}
	c.devices = make([]nvml.Device, 0, count)
	for i := 0; i < count; i++ {
		dev, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}
		c.devices = append(c.devices, dev)
	}
	c.ready = true
	return c
}

func (c *nvidiaCollector) Name() string    { return "gpu-nvidia" }
func (c *nvidiaCollector) Available() bool { return c.ready && len(c.devices) > 0 }

func (c *nvidiaCollector) Close() error {
	if !c.ready {
		return nil
	}
	if ret := nvml.Shutdown(); ret != nvml.SUCCESS {
		return fmt.Errorf("nvml shutdown: %s", nvml.ErrorString(ret))
	}
	return nil
}

func (c *nvidiaCollector) Collect(_ context.Context, into *model.Snapshot) error {
	for i, d := range c.devices {
		g := model.GPU{Index: i, Vendor: "nvidia"}

		if name, ret := d.GetName(); ret == nvml.SUCCESS {
			g.Name = name
		}
		if uuid, ret := d.GetUUID(); ret == nvml.SUCCESS {
			g.UUID = uuid
		}
		if util, ret := d.GetUtilizationRates(); ret == nvml.SUCCESS {
			g.UtilGPU = float64(util.Gpu)
			g.UtilMemory = float64(util.Memory)
		}
		if mem, ret := d.GetMemoryInfo(); ret == nvml.SUCCESS {
			g.MemTotal = mem.Total
			g.MemUsed = mem.Used
		}
		if t, ret := d.GetTemperature(nvml.TEMPERATURE_GPU); ret == nvml.SUCCESS {
			g.TempCelsius = float64(t)
		}
		if p, ret := d.GetPowerUsage(); ret == nvml.SUCCESS {
			g.PowerWatts = float64(p) / 1000.0
		}
		if pl, ret := d.GetPowerManagementLimit(); ret == nvml.SUCCESS {
			g.PowerLimitW = float64(pl) / 1000.0
		}
		if clk, ret := d.GetClockInfo(nvml.CLOCK_GRAPHICS); ret == nvml.SUCCESS {
			g.ClockCore = clk
		}
		if clk, ret := d.GetClockInfo(nvml.CLOCK_MEM); ret == nvml.SUCCESS {
			g.ClockMem = clk
		}
		if fan, ret := d.GetFanSpeed(); ret == nvml.SUCCESS {
			g.FanPercent = float64(fan)
		}
		if enc, _, ret := d.GetEncoderUtilization(); ret == nvml.SUCCESS {
			g.EncoderPct = float64(enc)
		}
		if dec, _, ret := d.GetDecoderUtilization(); ret == nvml.SUCCESS {
			g.DecoderPct = float64(dec)
		}
		if procs, ret := d.GetComputeRunningProcesses(); ret == nvml.SUCCESS {
			for _, p := range procs {
				name := ""
				if n, r := nvml.SystemGetProcessName(int(p.Pid)); r == nvml.SUCCESS {
					name = n
				}
				g.Processes = append(g.Processes, model.GPUProcess{
					PID:         int32(p.Pid),
					Name:        name,
					MemoryBytes: p.UsedGpuMemory,
				})
			}
		}
		into.GPUs = append(into.GPUs, g)
	}
	return nil
}
