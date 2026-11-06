package render

import (
	"fmt"
	"image/color"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/engine"
	"github.com/yufeiminds/nameary/internal/textutil"
)

// ConsoleOptions controls the progressive console output.
type ConsoleOptions struct {
	OnlyAvailable bool
	NoColor       bool
	ASCII         bool
	Light         bool
	Celebrate     bool
	Width         int
	Height        int
	// Live enables the animated, in-place console view. Only use it when Out
	// is a real terminal.
	Live bool
	Out  io.Writer
}

type consolePalette struct {
	noColor bool
	text    color.Color
	muted   color.Color
	dim     color.Color
	accent  color.Color
	green   color.Color
	rose    color.Color
	amber   color.Color
	red     color.Color
	indigo  color.Color
}

func newConsolePalette(noColor, light bool) consolePalette {
	if light {
		return consolePalette{
			noColor: noColor,
			text:    lipgloss.Color("#1F2937"),
			muted:   lipgloss.Color("#64748B"),
			dim:     lipgloss.Color("#94A3B8"),
			accent:  lipgloss.Color("#0D9488"),
			green:   lipgloss.Color("#059669"),
			rose:    lipgloss.Color("#E11D48"),
			amber:   lipgloss.Color("#D97706"),
			red:     lipgloss.Color("#DC2626"),
			indigo:  lipgloss.Color("#4F46E5"),
		}
	}
	return consolePalette{
		noColor: noColor,
		text:    lipgloss.Color("#E6EDF3"),
		muted:   lipgloss.Color("#7D8FA3"),
		dim:     lipgloss.Color("#46586A"),
		accent:  lipgloss.Color("#5EEAD4"),
		green:   lipgloss.Color("#34D399"),
		rose:    lipgloss.Color("#FB7185"),
		amber:   lipgloss.Color("#FBBF24"),
		red:     lipgloss.Color("#F87171"),
		indigo:  lipgloss.Color("#818CF8"),
	}
}

func (p consolePalette) style(c color.Color) lipgloss.Style {
	if p.noColor || c == nil {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(c)
}

func (p consolePalette) bold(c color.Color) lipgloss.Style {
	s := p.style(c)
	if !p.noColor {
		s = s.Bold(true)
	}
	return s
}

func (p consolePalette) statusColor(s check.Status) color.Color {
	if p.noColor {
		return nil
	}
	switch s {
	case check.StatusAvailable:
		return p.green
	case check.StatusTaken:
		return p.rose
	case check.StatusUnknown, check.StatusTimeout, check.StatusRateLimited:
		return p.amber
	case check.StatusError:
		return p.red
	case check.StatusSkipped, check.StatusCanceled:
		return p.dim
	default:
		return p.muted
	}
}

// RunConsole prints the logo and grouped skeleton first. On a normal terminal
// it updates the rows in place as checks settle; otherwise it streams grouped
// results as they become complete.
func RunConsole(events <-chan engine.Event, jobs []engine.Job, opts ConsoleOptions) (Result, error) {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Width <= 0 {
		opts.Width = 100
	}
	if opts.Width < 60 {
		opts.Width = 60
	}
	if opts.Height <= 0 {
		opts.Height = 40
	}

	p := newConsolePalette(opts.NoColor, opts.Light)
	start := time.Now()
	results := map[string]check.Result{}
	counts := map[check.Status]int{}
	done := 0
	last := ""
	name := ""
	if len(jobs) > 0 {
		name = jobs[0].Raw
	}

	if opts.Live && !opts.OnlyAvailable {
		if cols, blockH, ok := consoleLayout(jobs, opts.Width, opts.Height, p, opts.ASCII); ok {
			return runConsoleBlock(events, jobs, opts, p, start, cols, blockH)
		}
	}

	// Fallback: header + groups summary first, then grouped results at the end.
	header := consoleHeaderLines(jobs, opts.Width, p, opts.ASCII, true, false)
	fmt.Fprint(opts.Out, strings.Join(header, "\n")+"\n")

	status := &consoleStatus{
		out: opts.Out, live: opts.Live, p: p, ascii: opts.ASCII,
		width: opts.Width, name: name, total: len(jobs),
	}
	if opts.Live {
		fmt.Fprint(opts.Out, "\x1b[?25l")
		defer fmt.Fprint(opts.Out, "\x1b[?25h")
		status.draw(counts, 0, "", start, false)
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	if opts.Live {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := time.NewTicker(100 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					status.mu.Lock()
					status.draw(counts, done, last, start, false)
					status.mu.Unlock()
				}
			}
		}()
	}

	totalByCat := map[check.Category]int{}
	settledByCat := map[check.Category]int{}
	flushed := map[check.Category]bool{}
	for _, j := range jobs {
		totalByCat[j.Category]++
	}
	flush := func(cat check.Category) {
		if flushed[cat] {
			return
		}
		flushed[cat] = true
		lines := consoleGroupLines(jobs, results, cat, opts.Width, p, opts.ASCII, opts.OnlyAvailable)
		if len(lines) == 0 {
			return
		}
		block := strings.Join(lines, "\n") + "\n\n"
		if opts.Live {
			status.mu.Lock()
			status.clear()
			fmt.Fprint(opts.Out, block)
			status.draw(counts, done, last, start, false)
			status.mu.Unlock()
			return
		}
		fmt.Fprint(opts.Out, block)
	}

	for ev := range events {
		if ev.Kind != engine.EventSettled {
			continue
		}
		results[ev.Job.ID] = ev.Result
		counts[ev.Result.Status]++
		done++
		last = ev.Job.TargetLabel + " " + ev.Result.Status.String()
		settledByCat[ev.Job.Category]++
		if settledByCat[ev.Job.Category] == totalByCat[ev.Job.Category] {
			flush(ev.Job.Category)
		}
	}
	if opts.Live {
		close(stop)
		wg.Wait()
		status.mu.Lock()
		status.clear()
		status.mu.Unlock()
	}

	res := Result{Counts: counts, Total: len(jobs), Duration: time.Since(start)}
	for _, j := range jobs {
		if _, ok := results[j.ID]; ok {
			continue
		}
		st := check.StatusCanceled
		if j.Err != nil {
			st = check.StatusSkipped
		}
		res.Counts[st]++
	}

	for _, cat := range consoleCategories(jobs) {
		flush(cat)
	}
	fmt.Fprint(opts.Out, consoleSummary(res, p, opts.ASCII, opts.Celebrate, opts.Width))
	return res, nil
}

func runConsoleBlock(events <-chan engine.Event, jobs []engine.Job, opts ConsoleOptions, p consolePalette, start time.Time, cols, blockH int) (Result, error) {
	results := map[string]check.Result{}
	counts := map[check.Status]int{}
	done, frame := 0, 0
	last := ""

	var mu sync.Mutex
	draw := func(initial, finished bool) {
		lines := consoleBlockLines(jobs, results, counts, done, last, frame, opts.Width, cols, p, opts.ASCII, start, finished)
		if initial {
			fmt.Fprint(opts.Out, strings.Join(lines, "\n"))
			return
		}
		fmt.Fprint(opts.Out, "\x1b[?2026h")
		if blockH > 1 {
			fmt.Fprintf(opts.Out, "\x1b[%dA", blockH-1)
		}
		fmt.Fprint(opts.Out, "\r\x1b[J")
		fmt.Fprint(opts.Out, strings.Join(lines, "\n"))
		fmt.Fprint(opts.Out, "\x1b[?2026l")
	}

	mu.Lock()
	draw(true, false)
	mu.Unlock()
	fmt.Fprint(opts.Out, "\x1b[?25l")
	defer fmt.Fprint(opts.Out, "\x1b[?25h")

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(110 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				mu.Lock()
				frame++
				draw(false, false)
				mu.Unlock()
			}
		}
	}()

	for ev := range events {
		if ev.Kind != engine.EventSettled {
			continue
		}
		mu.Lock()
		results[ev.Job.ID] = ev.Result
		counts[ev.Result.Status]++
		done++
		last = ev.Job.TargetLabel + " " + ev.Result.Status.String()
		draw(false, false)
		mu.Unlock()
	}
	close(stop)
	wg.Wait()

	mu.Lock()
	for _, j := range jobs {
		if _, ok := results[j.ID]; ok {
			continue
		}
		st := check.StatusCanceled
		if j.Err != nil {
			st = check.StatusSkipped
		}
		counts[st]++
	}
	draw(false, true)
	mu.Unlock()
	fmt.Fprintln(opts.Out)
	return Result{Counts: counts, Total: len(jobs), Duration: time.Since(start)}, nil
}

func consoleLayout(jobs []engine.Job, width, height int, p consolePalette, ascii bool) (int, int, bool) {
	headerH := len(consoleHeaderLines(jobs, width, p, ascii, false, true))
	for cols := 1; cols <= 3; cols++ {
		cellW := (width - (cols-1)*2) / cols
		if cellW < 28 {
			continue
		}
		h := headerH
		for _, cat := range consoleCategories(jobs) {
			n := countCategory(jobs, cat)
			if n == 0 {
				continue
			}
			h += 1 + (n+cols-1)/cols
		}
		h++ // status line
		if h <= height {
			return cols, h, true
		}
	}
	return 0, 0, false
}

func consoleBlockLines(jobs []engine.Job, results map[string]check.Result, counts map[check.Status]int, done int, last string, frame int, width, cols int, p consolePalette, ascii bool, start time.Time, finished bool) []string {
	lines := consoleHeaderLines(jobs, width, p, ascii, false, true)
	for _, cat := range consoleCategories(jobs) {
		groupJobs := jobsByCategory(jobs, cat)
		if len(groupJobs) == 0 {
			continue
		}
		settled := 0
		for _, j := range groupJobs {
			if _, ok := results[j.ID]; ok {
				settled++
			}
		}
		left := p.bold(p.accent).Render(string(cat))
		right := p.style(p.dim).Render(fmt.Sprintf("%d/%d", settled, len(groupJobs)))
		lines = append(lines, joinLR(left, right, width))

		cellW := (width - (cols-1)*2) / cols
		for i := 0; i < len(groupJobs); i += cols {
			cells := make([]string, 0, cols)
			for c := 0; c < cols; c++ {
				idx := i + c
				if idx >= len(groupJobs) {
					cells = append(cells, fit("", cellW))
					continue
				}
				j := groupJobs[idx]
				r, ok := results[j.ID]
				cells = append(cells, fit(consoleCell(j, r, ok, cellW, p, ascii), cellW))
			}
			lines = append(lines, strings.Join(cells, p.style(p.dim).Render("  ")))
		}
	}
	lines = append(lines, consoleStatusLine(counts, done, len(jobs), last, frame, width, p, ascii, start, finished))
	return lines
}

func consoleCell(job engine.Job, r check.Result, ok bool, width int, p consolePalette, ascii bool) string {
	label := consoleLabel(job)
	if !ok {
		labelW := min(30, max(6, width-13))
		return "  " + p.style(p.dim).Render(consoleGlyph(check.StatusQueued, ascii)) + " " +
			p.style(p.dim).Render(padPlain(ansi.Truncate(label, labelW, "…"), labelW)) +
			"  " + p.style(p.dim).Render("waiting")
	}
	labelW := min(24, max(6, width-16))
	statusText := r.Status.String()
	if r.Cached {
		statusText += "*"
	}
	line := "  " + p.style(p.statusColor(r.Status)).Render(consoleGlyph(r.Status, ascii)) + " " +
		p.style(p.text).Render(padPlain(ansi.Truncate(label, labelW, "…"), labelW)) + "  " +
		p.style(p.statusColor(r.Status)).Render(padPlain(statusText, 10))
	if width >= 44 {
		meta := consoleMeta(r, ascii)
		if meta != "" {
			line += "  " + p.style(p.muted).Render(ansi.Truncate(meta, max(0, width-labelW-16), "…"))
		}
	}
	return ansi.Truncate(line, width, "…")
}

func consoleMeta(r check.Result, ascii bool) string {
	var parts []string
	if summary := textutil.Clean(r.Summary); summary != "" {
		parts = append(parts, summary)
	}
	if r.Status == check.StatusAvailable && r.Confidence == check.Likely {
		parts = append(parts, "likely")
	}
	if r.Cached {
		parts = append(parts, "cached")
	} else if r.Latency > 0 {
		parts = append(parts, r.Latency.Round(time.Millisecond).String())
	}
	sep := " · "
	if ascii {
		sep = " | "
	}
	return strings.Join(parts, sep)
}

func consoleStatusLine(counts map[check.Status]int, done, total int, last string, frame, width int, p consolePalette, ascii bool, start time.Time, finished bool) string {
	if finished {
		parts := []string{
			p.style(p.green).Render(fmt.Sprintf("✓ %d available", counts[check.StatusAvailable])),
			p.style(p.rose).Render(fmt.Sprintf("● %d taken", counts[check.StatusTaken])),
			p.style(p.amber).Render(fmt.Sprintf("! %d unknown", counts[check.StatusUnknown])),
		}
		if n := counts[check.StatusTimeout]; n > 0 {
			parts = append(parts, p.style(p.amber).Render(fmt.Sprintf("T %d timeout", n)))
		}
		if n := counts[check.StatusRateLimited]; n > 0 {
			parts = append(parts, p.style(p.amber).Render(fmt.Sprintf("R %d rate limit", n)))
		}
		if n := counts[check.StatusError]; n > 0 {
			parts = append(parts, p.style(p.red).Render(fmt.Sprintf("✕ %d error", n)))
		}
		if n := counts[check.StatusSkipped] + counts[check.StatusCanceled]; n > 0 {
			parts = append(parts, p.style(p.dim).Render(fmt.Sprintf("%d skipped", n)))
		}
		parts = append(parts, p.style(p.muted).Render(time.Since(start).Round(10*time.Millisecond).String()))
		return fit(strings.Join(parts, p.style(p.dim).Render("   ·   ")), width)
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	if ascii {
		frames = []string{"|", "/", "-", "\\"}
	}
	line := "  " + p.style(p.accent).Render(frames[frame%len(frames)]) +
		p.style(p.muted).Render(fmt.Sprintf("  %d/%d", done, total)) +
		p.style(p.green).Render(fmt.Sprintf("   ✓ %d", counts[check.StatusAvailable])) +
		p.style(p.rose).Render(fmt.Sprintf("   ● %d", counts[check.StatusTaken])) +
		p.style(p.amber).Render(fmt.Sprintf("   ! %d", counts[check.StatusUnknown]))
	if n := counts[check.StatusTimeout]; n > 0 {
		line += p.style(p.amber).Render(fmt.Sprintf("   T %d", n))
	}
	if n := counts[check.StatusRateLimited]; n > 0 {
		line += p.style(p.amber).Render(fmt.Sprintf("   R %d", n))
	}
	if last != "" {
		line += p.style(p.dim).Render("   " + last)
	}
	return fit(line, width)
}

type consoleStatus struct {
	mu    sync.Mutex
	out   io.Writer
	live  bool
	p     consolePalette
	ascii bool
	width int
	name  string
	total int
}

func (s *consoleStatus) draw(counts map[check.Status]int, done int, last string, start time.Time, finished bool) {
	if !s.live {
		return
	}
	fmt.Fprint(s.out, "\r\x1b[2K"+consoleStatusLine(counts, done, s.total, last, int(time.Since(start)/(100*time.Millisecond)), s.width, s.p, s.ascii, start, finished))
}

func (s *consoleStatus) clear() {
	if s.live {
		fmt.Fprint(s.out, "\r\x1b[2K")
	}
}

func consoleHeaderLines(jobs []engine.Job, width int, p consolePalette, ascii, includeGroups, compactLogo bool) []string {
	name := ""
	if len(jobs) > 0 {
		name = truncatePlain(textutil.Clean(jobs[0].Raw), max(12, width-24))
	}
	sources := map[string]bool{}
	counts := map[check.Category]int{}
	for _, j := range jobs {
		sources[j.TargetLabel] = true
		counts[j.Category]++
	}
	info := []string{
		p.bold(p.text).Render(name) + p.style(p.muted).Render("  name discover"),
		p.style(p.dim).Render(strings.Repeat(ruleChar(ascii), max(20, min(40, width-22)))),
		kv(p, "sources", p.style(p.text).Render(fmt.Sprintf("%d", len(sources))), ascii),
		kv(p, "checks", p.style(p.text).Render(fmt.Sprintf("%d", len(jobs))), ascii),
	}
	logo := consoleLogo(p, ascii, compactLogo)
	header := lipgloss.JoinHorizontal(lipgloss.Top, logo, "   ", lipgloss.JoinVertical(lipgloss.Left, info...))
	lines := strings.Split(header, "\n")
	if includeGroups {
		lines = append(lines, "")
		lines = append(lines, consoleGroups(counts, p, ascii))
		lines = append(lines, p.style(p.dim).Render(strings.Repeat(ruleChar(ascii), min(width, 78))))
	}
	return lines
}

func consoleGroups(counts map[check.Category]int, p consolePalette, ascii bool) string {
	sep := "   "
	if ascii {
		sep = " | "
	}
	var parts []string
	for _, cat := range []check.Category{check.CategoryDomains, check.CategoryPackages, check.CategoryHandles} {
		if counts[cat] == 0 {
			continue
		}
		parts = append(parts, p.bold(p.accent).Render(string(cat))+
			p.style(p.muted).Render(fmt.Sprintf(" %d", counts[cat])))
	}
	return strings.Join(parts, p.style(p.dim).Render(sep))
}

func consoleGroupLines(jobs []engine.Job, results map[string]check.Result, cat check.Category, width int, p consolePalette, ascii, onlyAvailable bool) []string {
	groupJobs := jobsByCategory(jobs, cat)
	if len(groupJobs) == 0 {
		return nil
	}
	settled := 0
	var rows []string
	for _, j := range groupJobs {
		r, ok := results[j.ID]
		if ok {
			settled++
		}
		if onlyAvailable && (!ok || r.Status != check.StatusAvailable) {
			continue
		}
		if !ok {
			st := check.StatusCanceled
			if j.Err != nil {
				st = check.StatusSkipped
			}
			r = check.Result{Status: st, Query: j.Query}
		}
		rows = append(rows, consoleRow(j, r, width, p, ascii))
	}
	if len(rows) == 0 {
		return nil
	}
	left := p.bold(p.accent).Render(string(cat))
	right := p.style(p.dim).Render(fmt.Sprintf("%d/%d", settled, len(groupJobs)))
	lines := []string{joinLR(left, right, width)}
	lines = append(lines, rows...)
	return lines
}

func consoleRow(job engine.Job, r check.Result, width int, p consolePalette, ascii bool) string {
	label := consoleLabel(job)
	labelW := min(38, max(10, width/3))
	statusText := r.Status.String()
	if r.Cached {
		statusText += "*"
	}
	line := "  " + p.style(p.statusColor(r.Status)).Render(consoleGlyph(r.Status, ascii)) + " " +
		p.style(p.text).Render(padPlain(ansi.Truncate(label, labelW, "…"), labelW)) + "  " +
		p.style(p.statusColor(r.Status)).Render(padPlain(statusText, 10))
	if meta := consoleMeta(r, ascii); meta != "" {
		line += "  " + p.style(p.muted).Render(ansi.Truncate(meta, max(0, width-labelW-20), "…"))
	}
	return ansi.Truncate(line, width, "…")
}

func consoleLabel(j engine.Job) string {
	label := textutil.Clean(j.Label)
	if j.Category != check.CategoryDomains {
		return textutil.Clean(j.TargetLabel) + "  " + label
	}
	return label
}

func consoleLogo(p consolePalette, ascii bool, compact bool) string {
	var lines []string
	if ascii {
		lines = []string{
			"+----------+",
			"|  N    N  |",
			"|  NN   N  |",
			"|  N N  N  |",
			"|  N  N N  |",
			"|  N   NN  |",
			"|  N    N  |",
			"+----------+",
		}
	} else {
		lines = []string{
			" ███╗   ██╗ ",
			" ████╗  ██║ ",
			" ██╔██╗ ██║ ",
			" ██║╚██╗██║ ",
			" ██║ ╚████║ ",
			" ╚═╝  ╚═══╝ ",
		}
	}
	var b strings.Builder
	if p.noColor {
		b.WriteString(strings.Join(lines, "\n"))
	} else {
		colors := lipgloss.Blend1D(len(lines), p.accent, p.indigo, p.rose)
		for i, line := range lines {
			b.WriteString(lipgloss.NewStyle().Foreground(colors[i]).Render(line))
			b.WriteString("\n")
		}
		if compact {
			return strings.TrimSuffix(b.String(), "\n")
		}
		b.WriteString(p.bold(p.muted).Render("  nameary  "))
		return b.String()
	}
	if !compact {
		b.WriteString("\n")
		b.WriteString(p.bold(p.muted).Render("  nameary  "))
	}
	return b.String()
}

func consoleSummary(res Result, p consolePalette, ascii, celebrate bool, width int) string {
	parts := []string{
		p.style(p.green).Render(fmt.Sprintf("✓ %d available", res.Counts[check.StatusAvailable])),
		p.style(p.rose).Render(fmt.Sprintf("● %d taken", res.Counts[check.StatusTaken])),
		p.style(p.amber).Render(fmt.Sprintf("! %d unknown", res.Counts[check.StatusUnknown])),
	}
	if n := res.Counts[check.StatusTimeout]; n > 0 {
		parts = append(parts, p.style(p.amber).Render(fmt.Sprintf("T %d timeout", n)))
	}
	if n := res.Counts[check.StatusRateLimited]; n > 0 {
		parts = append(parts, p.style(p.amber).Render(fmt.Sprintf("R %d rate limited", n)))
	}
	if n := res.Counts[check.StatusError]; n > 0 {
		parts = append(parts, p.style(p.red).Render(fmt.Sprintf("✕ %d error", n)))
	}
	if n := res.Counts[check.StatusSkipped] + res.Counts[check.StatusCanceled]; n > 0 {
		parts = append(parts, p.style(p.dim).Render(fmt.Sprintf("%d skipped", n)))
	}
	parts = append(parts, p.style(p.muted).Render(res.Duration.Round(10*time.Millisecond).String()))
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(p.style(p.dim).Render(strings.Repeat(ruleChar(ascii), min(width, 78))))
	b.WriteString("\n")
	b.WriteString(strings.Join(parts, p.style(p.dim).Render("   ·   ")))
	b.WriteString("\n")
	if celebrate && res.Counts[check.StatusTaken] == 0 && res.Counts[check.StatusAvailable] > 0 {
		mark := "✦"
		if ascii {
			mark = "*"
		}
		b.WriteString(p.style(p.green).Render("  " + mark + " all clear — every checked name is available"))
		b.WriteString("\n")
	}
	return b.String()
}

func consoleCategories(jobs []engine.Job) []check.Category {
	var out []check.Category
	seen := map[check.Category]bool{}
	for _, j := range jobs {
		if !seen[j.Category] {
			seen[j.Category] = true
			out = append(out, j.Category)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return catRank(out[i]) < catRank(out[j]) })
	return out
}

func catRank(c check.Category) int {
	switch c {
	case check.CategoryDomains:
		return 0
	case check.CategoryPackages:
		return 1
	case check.CategoryHandles:
		return 2
	default:
		return 3
	}
}

func countCategory(jobs []engine.Job, cat check.Category) int {
	n := 0
	for _, j := range jobs {
		if j.Category == cat {
			n++
		}
	}
	return n
}

func jobsByCategory(jobs []engine.Job, cat check.Category) []engine.Job {
	var out []engine.Job
	for _, j := range jobs {
		if j.Category == cat {
			out = append(out, j)
		}
	}
	return out
}

func kv(p consolePalette, key, value string, ascii bool) string {
	return p.style(p.muted).Render(padPlain(key, 11)) + value
}

func consoleGlyph(s check.Status, ascii bool) string {
	if ascii {
		switch s {
		case check.StatusAvailable:
			return "+"
		case check.StatusTaken:
			return "x"
		case check.StatusUnknown:
			return "!"
		case check.StatusTimeout:
			return "T"
		case check.StatusRateLimited:
			return "R"
		case check.StatusError:
			return "X"
		case check.StatusSkipped, check.StatusCanceled:
			return "-"
		default:
			return "."
		}
	}
	switch s {
	case check.StatusAvailable:
		return "✓"
	case check.StatusTaken:
		return "●"
	case check.StatusUnknown:
		return "!"
	case check.StatusTimeout:
		return "◷"
	case check.StatusRateLimited:
		return "↻"
	case check.StatusError:
		return "✕"
	case check.StatusSkipped, check.StatusCanceled:
		return "–"
	default:
		return "◌"
	}
}

func ruleChar(ascii bool) string {
	if ascii {
		return "-"
	}
	return "─"
}

func truncatePlain(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
}

func padPlain(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	if n := w - lipgloss.Width(s); n > 0 {
		s += strings.Repeat(" ", n)
	}
	return s
}

func joinLR(left, right string, w int) string {
	if w <= 0 {
		return ""
	}
	lw, rw := lipgloss.Width(left), lipgloss.Width(right)
	if lw+rw+1 > w {
		right = ansi.Truncate(right, max(0, w-lw-1), "…")
		rw = lipgloss.Width(right)
	}
	gap := w - lw - rw
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	if n := w - lipgloss.Width(s); n > 0 {
		s += strings.Repeat(" ", n)
	}
	return s
}
