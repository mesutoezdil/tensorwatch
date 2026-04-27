package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/mesutoezdil/tensorwatch/internal/model"
)

const (
	historyLen = 240
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
	compact    bool
	version    string
}

type Options struct {
	Compact bool
	Version string
}

func New(src <-chan model.Snapshot, opts Options) (*TUI, error) {
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
		cpuHistory: newRing(historyLen),
		gpuHistory: newRing(historyLen),
		memHistory: newRing(historyLen),
		compact:    opts.Compact,
		version:    opts.Version,
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
			t.draw(current)
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
				if e.Rune() == 'c' {
					t.compact = !t.compact
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
	if w < 80 || h < 16 {
		t.printf(0, 0, styleMuted(), "terminal too small (need at least 80x16)")
		t.screen.Show()
		return
	}

	t.drawHeader(s, w)

	bodyTop := 2
	footerHeight := 8
	if t.compact {
		footerHeight = 5
	}
	bodyHeight := h - bodyTop - footerHeight

	procRows := 0
	if !t.compact {
		procRows = minInt(10, len(s.Processes)+2)
		if procRows > 0 && bodyHeight < procRows+8 {
			procRows = 0
		}
	}
	splitHeight := bodyHeight - procRows

	t.drawCPU(s, 0, bodyTop, w/2-1, splitHeight)
	t.drawGPU(s, w/2+1, bodyTop, w-w/2-1, splitHeight)

	if procRows > 0 {
		t.drawProcesses(s, 0, bodyTop+splitHeight, w, procRows)
	}

	t.drawFooter(s, 0, h-footerHeight, w, footerHeight)
	t.screen.Show()
}

func (t *TUI) drawHeader(s model.Snapshot, w int) {
	left := fmt.Sprintf(" tensorwatch %s  %s  up %s ",
		t.version, s.Host.Hostname, formatUptime(s.Host.UptimeSec))
	mid := fmt.Sprintf(" load %.2f / %.2f / %.2f ",
		s.Host.Load1, s.Host.Load5, s.Host.Load15)
	right := " " + time.Now().Format("2006-01-02 15:04:05") + " "

	bar := left + strings.Repeat(" ", maxInt(0, (w-len(left)-len(mid)-len(right))/2)) + mid
	bar += strings.Repeat(" ", maxInt(0, w-len(bar)-len(right))) + right
	if len(bar) > w {
		bar = bar[:w]
	}
	t.printf(0, 0, styleHeader(), bar)
	t.printf(0, 1, styleMuted(), strings.Repeat("─", w))
}

func (t *TUI) drawCPU(s model.Snapshot, x, y, w, h int) {
	t.printf(x, y, styleSection(), "CPU")
	meta := strings.TrimSpace(s.CPU.ModelName)
	if meta == "" {
		meta = "unknown CPU"
	}
	meta = fmt.Sprintf("%s  ·  %dL/%dP", meta, s.CPU.LogicalCores, s.CPU.PhysicalCores)
	if s.CPU.TempCelsius > 0 {
		meta += fmt.Sprintf("  ·  %.1f°C", s.CPU.TempCelsius)
	}
	if s.CPU.FreqMHz >= 100 {
		meta += fmt.Sprintf("  ·  %.0f MHz", s.CPU.FreqMHz)
	}
	t.printf(x, y+1, styleMuted(), trim(meta, w))

	t.printf(x, y+3, tcell.StyleDefault, "overall ")
	t.drawBar(x+8, y+3, w-26, s.CPU.UsageOverall)
	t.printf(x+w-17, y+3, styleByThreshold(s.CPU.UsageOverall), fmt.Sprintf(" %5.1f%%", s.CPU.UsageOverall))
	if s.Peaks.CPU > 0 {
		t.printf(x+w-9, y+3, styleMuted(), fmt.Sprintf(" pk %3.0f%%", s.Peaks.CPU))
	}

	startY := y + 5
	maxRows := h - 5
	if maxRows < 1 {
		return
	}
	cores := s.CPU.UsagePerCore
	cols := 2
	if w < 50 {
		cols = 1
	}
	perCol := (len(cores) + cols - 1) / cols
	colW := (w - 2) / cols
	if colW < 18 {
		cols = 1
		colW = w
		perCol = len(cores)
	}
	for c := 0; c < cols; c++ {
		for r := 0; r < perCol; r++ {
			i := c*perCol + r
			if i >= len(cores) {
				break
			}
			if r >= maxRows {
				break
			}
			cx := x + c*colW
			row := startY + r
			t.printf(cx, row, styleMuted(), fmt.Sprintf("%2d ", i))
			t.drawBar(cx+3, row, colW-12, cores[i])
			t.printf(cx+colW-8, row, styleByThreshold(cores[i]), fmt.Sprintf(" %5.1f%%", cores[i]))
		}
	}
}

func (t *TUI) drawGPU(s model.Snapshot, x, y, w, h int) {
	t.printf(x, y, styleSection(), "GPU")
	if len(s.GPUs) == 0 {
		t.printf(x, y+2, styleMuted(), "no GPU collector active")
		t.printf(x, y+3, styleMuted(), "build with: go build -tags nvidia")
		if s.Peaks.WindowSec > 0 {
			t.printf(x, y+5, styleMuted(),
				fmt.Sprintf("peak window %s", formatDur(time.Duration(s.Peaks.WindowSec)*time.Second)))
		}
		return
	}
	row := y + 1
	for i, g := range s.GPUs {
		if row-y >= h-2 {
			t.printf(x, row, styleMuted(), fmt.Sprintf("(+%d more GPUs)", len(s.GPUs)-i))
			return
		}
		head := fmt.Sprintf("[%d] %s", g.Index, g.Name)
		t.printf(x, row, styleSubsection(), trim(head, w))
		row++
		t.printf(x, row, tcell.StyleDefault, "util  ")
		t.drawBar(x+6, row, w-26, g.UtilGPU)
		t.printf(x+w-19, row, styleByThreshold(g.UtilGPU), fmt.Sprintf(" %5.1f%%", g.UtilGPU))
		if s.Peaks.GPU > 0 {
			t.printf(x+w-11, row, styleMuted(), fmt.Sprintf(" pk %3.0f%%", s.Peaks.GPU))
		}
		row++

		memPct := 0.0
		if g.MemTotal > 0 {
			memPct = float64(g.MemUsed) / float64(g.MemTotal) * 100
		}
		t.printf(x, row, tcell.StyleDefault, "vram  ")
		t.drawBar(x+6, row, w-26, memPct)
		t.printf(x+w-19, row, styleByThreshold(memPct), fmt.Sprintf(" %5.1f%%", memPct))
		row++

		t.printf(x, row, styleMuted(),
			fmt.Sprintf("mem %s / %s   pwr %.0fW   tmp %.0f°C",
				humanBytes(g.MemUsed), humanBytes(g.MemTotal), g.PowerWatts, g.TempCelsius))
		row++

		if g.ClockCore > 0 {
			t.printf(x, row, styleMuted(),
				fmt.Sprintf("clk gfx %d MHz · mem %d MHz   fan %.0f%%",
					g.ClockCore, g.ClockMem, g.FanPercent))
			row++
		}
		row++
	}
}

func (t *TUI) drawProcesses(s model.Snapshot, x, y, w, h int) {
	t.printf(x, y, styleSection(), "PROCESSES")
	t.printf(x, y+1, styleMuted(),
		padRight("  PID  USER             CPU%   MEM%       RSS  COMMAND", w))
	maxRows := h - 2
	for i, p := range s.Processes {
		if i >= maxRows {
			break
		}
		user := trim(p.User, 14)
		cmd := trim(p.Command, w-46)
		line := fmt.Sprintf("%5d  %-14s %5.1f  %5.1f  %9s  %s",
			p.PID, user, p.CPUPct, p.MemPct, humanBytes(p.RSS), cmd)
		st := tcell.StyleDefault
		if p.CPUPct > 80 {
			st = st.Foreground(tcell.ColorRed)
		} else if p.CPUPct > 30 {
			st = st.Foreground(tcell.ColorYellow)
		}
		t.printf(x, y+2+i, st, padRight(line, w))
	}
}

func (t *TUI) drawFooter(s model.Snapshot, x, y, w, h int) {
	t.printf(x, y, styleSection(), "MEMORY")

	memBarW := w/2 - 26
	if memBarW < 10 {
		memBarW = 10
	}
	t.printf(x, y+1, tcell.StyleDefault, "ram   ")
	t.drawBar(x+6, y+1, memBarW, s.Memory.UsedPct)
	t.printf(x+6+memBarW, y+1, styleByThreshold(s.Memory.UsedPct),
		fmt.Sprintf(" %5.1f%%", s.Memory.UsedPct))
	t.printf(x+6+memBarW+8, y+1, styleMuted(),
		fmt.Sprintf("  %s / %s   pk %3.0f%%",
			humanBytes(s.Memory.UsedBytes), humanBytes(s.Memory.TotalBytes), s.Peaks.MemPct))

	swapPct := 0.0
	if s.Memory.SwapTotal > 0 {
		swapPct = float64(s.Memory.SwapUsed) / float64(s.Memory.SwapTotal) * 100
	}
	t.printf(x, y+2, tcell.StyleDefault, "swap  ")
	t.drawBar(x+6, y+2, memBarW, swapPct)
	t.printf(x+6+memBarW, y+2, styleByThreshold(swapPct), fmt.Sprintf(" %5.1f%%", swapPct))
	t.printf(x+6+memBarW+8, y+2, styleMuted(),
		fmt.Sprintf("  %s / %s",
			humanBytes(s.Memory.SwapUsed), humanBytes(s.Memory.SwapTotal)))

	if h >= 6 {
		t.printf(x, y+3, styleSection(), "HISTORY")
		sparkW := w - 8
		t.printf(x, y+4, tcell.StyleDefault.Foreground(tcell.ColorGreen),
			"cpu   "+sparkline(t.cpuHistory.ordered(), sparkW))
		t.printf(x, y+5, tcell.StyleDefault.Foreground(tcell.ColorAqua),
			"gpu   "+sparkline(t.gpuHistory.ordered(), sparkW))
		t.printf(x, y+6, tcell.StyleDefault.Foreground(tcell.ColorPurple),
			"mem   "+sparkline(t.memHistory.ordered(), sparkW))
	}

	hint := " q quit  ·  c " + ifThen(t.compact, "expand", "compact") +
		"  ·  pk = " + formatDur(time.Duration(s.Peaks.WindowSec)*time.Second) + " peak "
	t.printf(maxInt(0, w-len(hint)), y+h-1, styleMuted(), hint)
}

func (t *TUI) drawBar(x, y, width int, pct float64) {
	if width < 4 {
		return
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	style := styleByThreshold(pct)
	filled := int(float64(width-2) * pct / 100)
	t.screen.SetContent(x, y, '[', nil, styleMuted())
	for i := 0; i < width-2; i++ {
		ch := ' '
		st := tcell.StyleDefault
		if i < filled {
			ch = '█'
			st = style
		}
		t.screen.SetContent(x+1+i, y, ch, nil, st)
	}
	t.screen.SetContent(x+width-1, y, ']', nil, styleMuted())
}

func (t *TUI) printf(x, y int, style tcell.Style, msg string) {
	col := x
	for _, r := range msg {
		t.screen.SetContent(col, y, r, nil, style)
		col++
	}
}

func styleHeader() tcell.Style {
	return tcell.StyleDefault.
		Background(tcell.ColorDarkSlateGray).
		Foreground(tcell.ColorWhite).
		Bold(true)
}

func styleSection() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorAqua).Bold(true)
}

func styleSubsection() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true)
}

func styleMuted() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorGray)
}

func styleByThreshold(pct float64) tcell.Style {
	switch {
	case pct >= 90:
		return tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
	case pct >= 60:
		return tcell.StyleDefault.Foreground(tcell.ColorYellow)
	default:
		return tcell.StyleDefault.Foreground(tcell.ColorGreen)
	}
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
		return fmt.Sprintf("%dd%dh%dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh%dm", hours, mins)
}

func formatDur(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
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

func ifThen(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
