package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

type ringBuffer struct {
	values []float64
	cap    int
	idx    int
	full   bool
}

func newRing(cap int) *ringBuffer { return &ringBuffer{cap: cap, values: make([]float64, 0, cap)} }

func (r *ringBuffer) push(v float64) {
	if len(r.values) < r.cap {
		r.values = append(r.values, v)
		return
	}
	r.values[r.idx] = v
	r.idx = (r.idx + 1) % r.cap
	r.full = true
}

func (r *ringBuffer) ordered() []float64 {
	if !r.full {
		out := make([]float64, len(r.values))
		copy(out, r.values)
		return out
	}
	out := make([]float64, 0, r.cap)
	out = append(out, r.values[r.idx:]...)
	out = append(out, r.values[:r.idx]...)
	return out
}

type TUI struct {
	screen     tcell.Screen
	src        <-chan model.Snapshot
	cpuHistory *ringBuffer
	gpuHistory *ringBuffer
	memHistory *ringBuffer
}

func New(src <-chan model.Snapshot) (*TUI, error) {
	scr, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	if err := scr.Init(); err != nil {
		return nil, err
	}
	scr.SetStyle(tcell.StyleDefault)
	scr.Clear()
	return &TUI{
		screen:     scr,
		src:        src,
		cpuHistory: newRing(120),
		gpuHistory: newRing(120),
		memHistory: newRing(120),
	}, nil
}

func (t *TUI) Run(ctx context.Context) error {
	defer t.screen.Fini()

	events := make(chan tcell.Event, 16)
	go func() {
		for {
			ev := t.screen.PollEvent()
			if ev == nil {
				return
			}
			events <- ev
		}
	}()

	var current model.Snapshot
	have := false
	redraw := time.NewTicker(250 * time.Millisecond)
	defer redraw.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case s := <-t.src:
			current = s
			have = true
			t.cpuHistory.push(s.CPU.UsageOverall)
			t.memHistory.push(s.Memory.UsedPct)
			if len(s.GPUs) > 0 {
				t.gpuHistory.push(s.GPUs[0].UtilGPU)
			} else {
				t.gpuHistory.push(0)
			}
			if have {
				t.draw(current)
			}
		case <-redraw.C:
			if have {
				t.draw(current)
			}
		case ev := <-events:
			switch e := ev.(type) {
			case *tcell.EventKey:
				if e.Key() == tcell.KeyEscape || e.Key() == tcell.KeyCtrlC || e.Rune() == 'q' {
					return nil
				}
			case *tcell.EventResize:
				t.screen.Sync()
			}
		}
	}
}

func (t *TUI) draw(s model.Snapshot) {
	t.screen.Clear()
	w, h := t.screen.Size()
	if w < 60 || h < 10 {
		t.printf(0, 0, tcell.StyleDefault, "terminal too small")
		t.screen.Show()
		return
	}

	t.drawHeader(s, w)
	t.drawCPU(s, 0, 2, w/2-1, h-6)
	t.drawGPU(s, w/2, 2, w-w/2, h-6)
	t.drawFooter(s, 0, h-4, w)
	t.screen.Show()
}

func (t *TUI) drawHeader(s model.Snapshot, w int) {
	title := fmt.Sprintf("tensorwatch  %s  uptime %s", s.Host.Hostname, formatUptime(s.Host.UptimeSec))
	right := time.Now().Format("2006-01-02 15:04:05")
	t.printf(0, 0, headerStyle(), padRight(title, w-len(right))+right)
	t.printf(0, 1, tcell.StyleDefault.Foreground(tcell.ColorGray), strings.Repeat("─", w))
}

func (t *TUI) drawCPU(s model.Snapshot, x, y, w, h int) {
	t.printf(x, y, sectionStyle(), padRight("CPU", w))
	t.printf(x, y+1, tcell.StyleDefault, fmt.Sprintf("%-14s %s", "model", trim(s.CPU.ModelName, w-15)))
	t.printf(x, y+2, tcell.StyleDefault, fmt.Sprintf("%-14s %d logical / %d physical", "cores", s.CPU.LogicalCores, s.CPU.PhysicalCores))
	if s.CPU.FreqMHz >= 100 {
		t.printf(x, y+3, tcell.StyleDefault, fmt.Sprintf("%-14s %.0f MHz", "frequency", s.CPU.FreqMHz))
	}
	if s.CPU.TempCelsius > 0 {
		t.printf(x, y+4, tcell.StyleDefault, fmt.Sprintf("%-14s %.1f °C", "temperature", s.CPU.TempCelsius))
	}

	t.printf(x, y+6, tcell.StyleDefault, fmt.Sprintf("overall  %s %5.1f%%", bar(s.CPU.UsageOverall, w-19), s.CPU.UsageOverall))
	t.printf(x, y+7, tcell.StyleDefault, "history  "+sparkline(t.cpuHistory.ordered(), w-10))

	startY := y + 9
	maxRows := h - 10
	if maxRows < 1 {
		return
	}
	for i, u := range s.CPU.UsagePerCore {
		if i >= maxRows {
			break
		}
		label := fmt.Sprintf("%2d", i)
		t.printf(x, startY+i, tcell.StyleDefault, fmt.Sprintf("%s  %s %5.1f%%", label, bar(u, w-13), u))
	}
}

func (t *TUI) drawGPU(s model.Snapshot, x, y, w, h int) {
	t.printf(x, y, sectionStyle(), padRight("GPU", w))
	if len(s.GPUs) == 0 {
		t.printf(x, y+2, tcell.StyleDefault.Foreground(tcell.ColorGray),
			"no GPU collector active")
		t.printf(x, y+3, tcell.StyleDefault.Foreground(tcell.ColorGray),
			"build with: go build -tags nvidia")
		return
	}
	row := y + 1
	for _, g := range s.GPUs {
		head := fmt.Sprintf("[%d] %s", g.Index, g.Name)
		t.printf(x, row, tcell.StyleDefault.Bold(true), trim(head, w))
		row++
		t.printf(x, row, tcell.StyleDefault, fmt.Sprintf("util     %s %5.1f%%", bar(g.UtilGPU, w-19), g.UtilGPU))
		row++
		memPct := 0.0
		if g.MemTotal > 0 {
			memPct = float64(g.MemUsed) / float64(g.MemTotal) * 100
		}
		t.printf(x, row, tcell.StyleDefault, fmt.Sprintf("memory   %s %5.1f%%", bar(memPct, w-19), memPct))
		row++
		t.printf(x, row, tcell.StyleDefault, fmt.Sprintf("vram     %s / %s", humanBytes(g.MemUsed), humanBytes(g.MemTotal)))
		row++
		t.printf(x, row, tcell.StyleDefault, fmt.Sprintf("temp     %.0f °C   power %.0f W", g.TempCelsius, g.PowerWatts))
		row++
		if g.ClockCore > 0 {
			t.printf(x, row, tcell.StyleDefault, fmt.Sprintf("clocks   gfx %d MHz / mem %d MHz", g.ClockCore, g.ClockMem))
			row++
		}
		row++
		if row-y >= h-1 {
			return
		}
	}
	if row-y < h-2 {
		t.printf(x, row, tcell.StyleDefault, "history  "+sparkline(t.gpuHistory.ordered(), w-10))
	}
}

func (t *TUI) drawFooter(s model.Snapshot, x, y, w int) {
	t.printf(x, y, sectionStyle(), padRight("MEMORY", w))
	t.printf(x, y+1, tcell.StyleDefault,
		fmt.Sprintf("ram   %s %5.1f%%   %s / %s",
			bar(s.Memory.UsedPct, w/2-22), s.Memory.UsedPct,
			humanBytes(s.Memory.UsedBytes), humanBytes(s.Memory.TotalBytes)))
	swapPct := 0.0
	if s.Memory.SwapTotal > 0 {
		swapPct = float64(s.Memory.SwapUsed) / float64(s.Memory.SwapTotal) * 100
	}
	t.printf(x, y+2, tcell.StyleDefault,
		fmt.Sprintf("swap  %s %5.1f%%   %s / %s",
			bar(swapPct, w/2-22), swapPct,
			humanBytes(s.Memory.SwapUsed), humanBytes(s.Memory.SwapTotal)))
	hint := fmt.Sprintf("load %.2f / %.2f / %.2f   q,Esc to quit", s.Host.Load1, s.Host.Load5, s.Host.Load15)
	t.printf(x, y+3, tcell.StyleDefault.Foreground(tcell.ColorGray), padRight(hint, w))
}

func (t *TUI) printf(x, y int, style tcell.Style, msg string) {
	for i, r := range msg {
		t.screen.SetContent(x+i, y, r, nil, style)
	}
}

func headerStyle() tcell.Style {
	return tcell.StyleDefault.Background(tcell.ColorDarkSlateGray).Foreground(tcell.ColorWhite).Bold(true)
}

func sectionStyle() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorAqua).Bold(true)
}

func bar(pct float64, width int) string {
	if width < 4 {
		return strings.Repeat(" ", width)
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(float64(width-2) * pct / 100)
	var sb strings.Builder
	sb.WriteByte('[')
	sb.WriteString(strings.Repeat("█", filled))
	sb.WriteString(strings.Repeat(" ", width-2-filled))
	sb.WriteByte(']')
	return sb.String()
}

func sparkline(values []float64, width int) string {
	if width < 1 {
		return ""
	}
	glyphs := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	if len(values) > width {
		values = values[len(values)-width:]
	}
	var sb strings.Builder
	for _, v := range values {
		idx := int(v / 100 * float64(len(glyphs)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(glyphs) {
			idx = len(glyphs) - 1
		}
		sb.WriteRune(glyphs[idx])
	}
	for sb.Len() < width {
		sb.WriteRune(' ')
	}
	return sb.String()
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatUptime(sec uint64) string {
	d := time.Duration(sec) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}

func trim(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return s[:w]
	}
	return s[:w-1] + "…"
}
