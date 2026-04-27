package exporter

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type CSVSink struct {
	mu     sync.Mutex
	writer *csv.Writer
	file   *os.File
	header bool
}

func OpenCSV(path string) (*CSVSink, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	st, _ := f.Stat()
	c := &CSVSink{
		file:   f,
		writer: csv.NewWriter(f),
		header: st != nil && st.Size() == 0,
	}
	return c, nil
}

func (c *CSVSink) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writer.Flush()
	return c.file.Close()
}

func (c *CSVSink) Consume(ctx context.Context, src <-chan model.Snapshot) {
	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			c.writer.Flush()
			c.mu.Unlock()
			return
		case s, ok := <-src:
			if !ok {
				return
			}
			c.write(s)
		}
	}
}

func (c *CSVSink) write(s model.Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.header {
		_ = c.writer.Write([]string{
			"taken_unix", "cpu_overall_pct", "cpu_temp_c", "mem_used_pct",
			"swap_used_bytes", "gpu_index", "gpu_util_pct", "gpu_mem_used_bytes",
			"gpu_temp_c", "gpu_power_w",
		})
		c.header = false
	}
	ts := strconv.FormatInt(s.Taken.Unix(), 10)
	if len(s.GPUs) == 0 {
		_ = c.writer.Write([]string{
			ts,
			f(s.CPU.UsageOverall),
			f(s.CPU.TempCelsius),
			f(s.Memory.UsedPct),
			strconv.FormatUint(s.Memory.SwapUsed, 10),
			"", "", "", "", "",
		})
	} else {
		for _, g := range s.GPUs {
			_ = c.writer.Write([]string{
				ts,
				f(s.CPU.UsageOverall),
				f(s.CPU.TempCelsius),
				f(s.Memory.UsedPct),
				strconv.FormatUint(s.Memory.SwapUsed, 10),
				strconv.Itoa(g.Index),
				f(g.UtilGPU),
				strconv.FormatUint(g.MemUsed, 10),
				f(g.TempCelsius),
				f(g.PowerWatts),
			})
		}
	}
	c.writer.Flush()
}

func f(v float64) string { return fmt.Sprintf("%.2f", v) }
