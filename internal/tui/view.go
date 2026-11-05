package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/textutil"
)

func (m *model) View() tea.View {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	w = max(w, 20)
	h = max(h, 6)

	var content string
	if m.phase == phaseInput {
		content = m.screen(m.inputBody(w, h), w, h)
	} else {
		content = m.mainScreen(w, h)
	}
	if m.detail != nil {
		content = m.overlay(content, m.detailPanel())
	}
	if m.helpOpen {
		content = m.overlay(content, m.helpPanel())
	}

	v := tea.NewView(content)
	v.AltScreen = true
	if m.phase != phaseInput {
		v.WindowTitle = fmt.Sprintf("nameary · %s · %d/%d", textutil.Clean(m.name), m.settled, len(m.rows))
		v.ProgressBar = tea.NewProgressBar(tea.ProgressBarDefault, int(m.progressRatio()*100))
	}
	return v
}

func (m *model) screen(lines []string, w, h int) string {
	out := make([]string, h)
	for i := 0; i < h; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out[i] = fit(line, w)
	}
	return strings.Join(out, "\n")
}

func (m *model) mainScreen(w, h int) string {
	contentW := max(1, w-2)
	header := m.headerLines(contentW)
	footer := m.footerLines(contentW)
	bodyH := h - len(header) - len(footer)
	if bodyH < 1 {
		bodyH = 1
	}
	body := m.bodyLines(contentW, bodyH)
	lines := make([]string, 0, len(header)+len(body)+len(footer))
	lines = append(lines, header...)
	lines = append(lines, body...)
	lines = append(lines, footer...)
	padded := make([]string, len(lines))
	for i, line := range lines {
		padded[i] = "  " + fit(line, contentW)
	}
	return m.screen(padded, w, h)
}

func (m *model) headerLines(w int) []string {
	brand := m.logo()
	left := brand + m.theme.style(m.theme.dim).Render("  /  name discover")
	right := ""
	if m.phase == phaseRunning {
		right = m.theme.style(m.theme.muted).Render(fmt.Sprintf("%d/%d", m.settled, len(m.rows))) +
			"  " + m.spinner.View()
	} else if m.phase == phaseDone {
		dur := time.Since(m.started)
		if !m.finished.IsZero() {
			dur = m.finished.Sub(m.started)
		}
		right = m.theme.style(m.theme.green).Render("done") +
			m.theme.style(m.theme.muted).Render(" · "+dur.Round(10*time.Millisecond).String())
	}
	name := m.theme.bold(m.theme.text).Render(textutil.Clean(m.name))
	sources := m.theme.style(m.theme.muted).Render(fmt.Sprintf("%d sources", m.sourceCount()))
	return []string{
		"",
		joinLR(left, right, w),
		joinLR(name, sources, w),
		m.progressLine(w),
		m.statsLine(w),
		m.rule(w),
	}
}

func (m *model) sourceCount() int {
	seen := map[string]bool{}
	for _, r := range m.rows {
		seen[r.job.TargetLabel] = true
	}
	return len(seen)
}

func (m *model) rule(w int) string {
	ch := "─"
	if m.opts.ASCII {
		ch = "-"
	}
	return m.theme.style(m.theme.dim).Render(strings.Repeat(ch, max(0, w)))
}

func (m *model) bodyLines(w, h int) []string {
	if w >= 96 {
		sideW := min(40, max(30, w/3))
		listW := w - sideW - 3
		if listW < 40 {
			return m.listLines(w)
		}
		list := m.listLines(listW)
		side := m.sideLines(sideW)
		for len(side) > 0 && side[len(side)-1] == "" {
			side = side[:len(side)-1]
		}
		contentH := max(len(list), len(side))
		top := 0
		if contentH < h {
			top = (h - contentH) / 2
		}
		lines := make([]string, 0, h)
		for i := 0; i < h; i++ {
			left, right := "", ""
			if i >= top && i-top < len(list) {
				left = list[i-top]
			}
			if i >= top && i-top < len(side) {
				right = side[i-top]
			}
			lines = append(lines, fit(left, listW)+m.theme.style(m.theme.dim).Render(" │ ")+fit(right, sideW))
		}
		return lines
	}
	list := m.listLines(w)
	if len(list) >= h {
		return list
	}
	top := (h - len(list)) / 2
	out := make([]string, 0, h)
	for i := 0; i < top; i++ {
		out = append(out, "")
	}
	out = append(out, list...)
	for len(out) < h {
		out = append(out, "")
	}
	return out
}

func (m *model) frame(title string, body []string, w, h int) string {
	if w < 4 {
		w = 4
	}
	if h < 3 {
		h = 3
	}
	innerW, innerH := w-2, h-2
	border := m.theme.borderStyle()

	title = ansi.Truncate(title, max(0, innerW-6), m.theme.ellipsis())
	side := "│"
	top := ""
	bottom := border.Render("╰" + strings.Repeat("─", innerW) + "╯")
	if m.opts.ASCII {
		side = border.Render("|")
		bottom = border.Render("+" + strings.Repeat("-", innerW) + "+")
	}
	if title == "" {
		if m.opts.ASCII {
			top = border.Render("+" + strings.Repeat("-", innerW) + "+")
		} else {
			top = border.Render("╭" + strings.Repeat("─", innerW) + "╮")
		}
	} else {
		dash := innerW - lipgloss.Width(title) - 3
		if dash < 0 {
			dash = 0
		}
		if m.opts.ASCII {
			top = border.Render("+- ") +
				m.theme.bold(m.theme.text).Render(title) +
				border.Render(" "+strings.Repeat("-", dash)+"+")
		} else {
			top = border.Render("╭─ ") +
				m.theme.bold(m.theme.text).Render(title) +
				border.Render(" "+strings.Repeat("─", dash)+"╮")
		}
	}

	lines := make([]string, 0, h)
	lines = append(lines, top)
	for i := 0; i < innerH; i++ {
		line := ""
		if i < len(body) {
			line = body[i]
		}
		lines = append(lines, side+fit(line, innerW)+side)
	}
	lines = append(lines, bottom)
	return strings.Join(lines, "\n")
}

func (m *model) inputBody(w, h int) []string {
	logo := m.logo()
	tagline := m.theme.style(m.theme.muted).Render("find a name before you commit.")
	var lines []string
	lines = append(lines, "")
	lines = append(lines, center(logo, w))
	lines = append(lines, center(tagline, w))
	lines = append(lines, "")
	divider := "─  name  ─"
	if m.opts.ASCII {
		divider = "--- name ---"
	}
	lines = append(lines, center(m.theme.style(m.theme.dim).Render(divider), w))
	lines = append(lines, "")

	inputBox := m.input.View()
	if m.inputErr != nil {
		lines = append(lines, center(m.theme.style(m.theme.red).Render("✕ "+textutil.Clean(m.inputErr.Error())), w))
	} else {
		lines = append(lines, "")
	}
	lines = append(lines, center(inputBox, w))
	lines = append(lines, "")

	sep := " · "
	if m.opts.ASCII {
		sep = " | "
	}
	if len(m.opts.Targets) > 0 {
		targets := strings.Join(m.opts.Targets, sep)
		if len(m.opts.Targets) > 8 {
			targets = fmt.Sprintf("%d targets selected", len(m.opts.Targets))
		}
		lines = append(lines, center(m.theme.style(m.theme.muted).Render("targets  "+textutil.Clean(targets)), w))
	}
	if len(m.opts.TLDs) > 0 {
		lines = append(lines, center(m.theme.style(m.theme.dim).Render("domains  "+textutil.Clean(strings.Join(m.opts.TLDs, sep))), w))
	}
	lines = append(lines, "")
	footer := "⏎ discover   ? help   ctrl+c quit"
	if m.opts.ASCII {
		footer = "enter discover   ? help   ctrl+c quit"
	}
	lines = append(lines, center(m.theme.style(m.theme.dim).Render(footer), w))

	if len(lines) < h {
		pad := (h - len(lines)) / 2
		if pad > 0 {
			lines = append(make([]string, pad), lines...)
		}
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return lines
}

func (m *model) logo() string {
	word := "nameary"
	if m.opts.NoColor {
		return m.theme.bold(m.theme.text).Render(word)
	}
	runes := []rune(word)
	colors := lipgloss.Blend1D(len(runes)+8, m.theme.accent, m.theme.indigo, m.theme.rose)
	phase := 0
	if !m.opts.ReducedMotion && !m.anim.logoStart.IsZero() {
		if time.Since(m.anim.logoStart) < 900*time.Millisecond {
			phase = m.anim.frame / 2
		}
	}
	var b strings.Builder
	for i, r := range runes {
		c := colors[(i+phase)%len(colors)]
		b.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render(string(r)))
	}
	return b.String()
}

func (m *model) sideLines(w int) []string {
	h := m.listHeight()
	lines := []string{
		m.theme.bold(m.theme.accent).Render("SELECTED"),
	}
	r := m.currentRow()
	if r == nil {
		lines = append(lines, m.theme.style(m.theme.dim).Render("move the cursor over a result"))
	} else {
		status := r.status
		if status == check.StatusQueued && r.job.Err != nil {
			status = check.StatusSkipped
		}
		title := textutil.Clean(r.job.Label)
		if r.job.Category != check.CategoryDomains {
			title = textutil.Clean(r.job.TargetLabel) + "  " + title
		}
		lines = append(lines, m.theme.bold(m.theme.text).Render(truncateText(title, max(8, w-2))))
		conf := r.result.Confidence
		if !status.IsFinal() {
			conf = check.Low
		}
		lines = append(lines,
			m.theme.style(m.theme.statusColor(status)).Render(m.theme.glyph(status, ""))+
				" "+m.theme.style(m.theme.statusColor(status)).Render(status.String())+
				m.theme.style(m.theme.muted).Render("  ·  "+conf.String()))
		if summary := textutil.Clean(r.result.Summary); summary != "" {
			lines = appendWrapped(lines, m.theme.style(m.theme.muted), summary, w-2)
		}
		for _, f := range r.result.Fields {
			if f.Value == "" || len(lines) >= h-4 {
				break
			}
			lines = appendKV(lines, m, textutil.Clean(f.Key), textutil.Clean(f.Value), w)
		}
		if r.result.SourceURL != "" && len(lines) < h-3 {
			lines = append(lines,
				m.theme.style(m.theme.muted).Render(padPlain("source", 9))+" "+
					m.theme.style(m.theme.dim).Render(truncateText(textutil.Clean(r.result.SourceURL), max(6, w-12))))
		}
	}

	if len(lines) < h-2 {
		lines = append(lines, "")
		lines = append(lines, m.theme.bold(m.theme.accent).Render("AVAILABLE"))
		var available []*row
		for _, row := range m.rows {
			if row.status == check.StatusAvailable {
				available = append(available, row)
			}
		}
		if len(available) == 0 {
			text := "none yet"
			if m.phase == phaseDone {
				text = "no available results"
			}
			lines = append(lines, m.theme.style(m.theme.dim).Render(text))
		}
		for _, row := range available {
			if len(lines) >= h {
				break
			}
			label := textutil.Clean(row.job.Label)
			if row.job.Category != check.CategoryDomains {
				label = textutil.Clean(row.job.TargetLabel) + "  " + label
			}
			lines = append(lines,
				m.theme.style(m.theme.green).Render(m.theme.glyph(check.StatusAvailable, "")+" ")+
					m.theme.style(m.theme.text).Render(truncateText(label, max(6, w-2))))
		}
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return lines
}

func appendKV(lines []string, m *model, key, value string, w int) []string {
	prefix := m.theme.style(m.theme.muted).Render(padPlain(key, 9))
	wrapped := wrapText(value, max(8, w-12))
	parts := strings.Split(wrapped, "\n")
	if len(parts) == 0 {
		return lines
	}
	lines = append(lines, prefix+" "+m.theme.style(m.theme.text).Render(parts[0]))
	for _, part := range parts[1:] {
		lines = append(lines, strings.Repeat(" ", 10)+m.theme.style(m.theme.text).Render(part))
	}
	return lines
}

func appendWrapped(lines []string, style lipgloss.Style, text string, w int) []string {
	for _, line := range strings.Split(wrapText(text, max(8, w)), "\n") {
		lines = append(lines, style.Render(line))
	}
	return lines
}

func (m *model) progressLine(w int) string {
	pct := m.progressRatio()
	barW := max(10, w-8)
	var bar string
	if m.opts.NoColor || m.opts.ASCII {
		filled := int(float64(barW) * pct)
		bar = "[" + strings.Repeat("#", filled) + strings.Repeat("-", max(0, barW-filled)) + "]"
	} else if m.opts.ReducedMotion {
		bar = m.prog.ViewAs(pct)
	} else {
		bar = m.prog.View()
	}
	pctText := m.theme.style(m.theme.muted).Render(fmt.Sprintf("%3.0f%%", pct*100))
	return joinLR(bar, pctText, w)
}

func (m *model) statsLine(w int) string {
	dot := func(s check.Status) string {
		return m.theme.style(m.theme.statusColor(s)).Render(m.theme.glyph(s, ""))
	}
	plain := func(s check.Status, n int, label string) string {
		return dot(s) + " " + m.theme.style(m.theme.text).Render(fmt.Sprintf("%d", n)) + " " + m.theme.style(m.theme.muted).Render(label)
	}
	inFlight := len(m.rows) - m.settled
	parts := []string{
		plain(check.StatusAvailable, m.counts[check.StatusAvailable], "available"),
		plain(check.StatusTaken, m.counts[check.StatusTaken], "taken"),
		plain(check.StatusUnknown, m.counts[check.StatusUnknown], "unknown"),
	}
	if inFlight > 0 {
		parts = append(parts, m.theme.style(m.theme.accent).Render("◐ ")+
			m.theme.style(m.theme.text).Render(fmt.Sprintf("%d", inFlight))+" "+
			m.theme.style(m.theme.muted).Render("in flight"))
	}
	if n := m.counts[check.StatusCanceled] + m.counts[check.StatusSkipped]; n > 0 {
		parts = append(parts, m.theme.style(m.theme.dim).Render(fmt.Sprintf("%d skipped", n)))
	}
	sep := "  ·  "
	if m.opts.ASCII {
		sep = "  |  "
	}
	return fit(strings.Join(parts, m.theme.style(m.theme.dim).Render(sep)), w)
}

func (m *model) listLines(w int) []string {
	var lines []string
	if len(m.items) == 0 {
		lines = append(lines, m.theme.style(m.theme.dim).Render("  no results match this filter"))
	} else {
		for _, it := range m.visibleItems() {
			if it.kind == itemHeader {
				lines = append(lines, m.groupLine(it, w))
				continue
			}
			if it.row >= 0 && it.row < len(m.rows) {
				lines = append(lines, m.rowLine(m.rows[it.row], w, m.offset+len(lines) == m.cursor))
			}
		}
	}
	if len(lines) > m.listHeight() {
		lines = lines[:m.listHeight()]
	}
	return lines
}

func (m *model) groupLine(it item, w int) string {
	settled, total := m.groupStats(it.group)
	left := m.theme.bold(m.theme.accent).Render(string(it.group))
	right := m.theme.style(m.theme.dim).Render(fmt.Sprintf("%d/%d", settled, total))
	return joinLR(left, right, w)
}

func (m *model) rowLine(r *row, w int, selected bool) string {
	compact := w < 64
	status := r.status
	if status == check.StatusQueued && r.job.Err != nil {
		status = check.StatusSkipped
	}

	cursor := " "
	if selected {
		cursor = m.theme.style(m.theme.accent).Render("▌")
		if m.opts.ASCII {
			cursor = m.theme.style(m.theme.accent).Render(">")
		}
	}
	spinnerFrame := ""
	if status == check.StatusChecking {
		spinnerFrame = m.spinner.View()
	}
	glyph := m.theme.style(m.theme.statusColor(status)).Render(m.theme.glyph(status, spinnerFrame))

	nameW := 22
	statusW := 10
	if compact {
		nameW = max(10, w-20)
	} else {
		nameW = min(40, max(18, w/3))
	}
	nameText := textutil.Clean(r.job.Label)
	if compact && r.job.Category == check.CategoryPackages {
		nameText = textutil.Clean(r.job.TargetLabel) + " " + nameText
	}
	if nameW < len(nameText) {
		nameText = ansi.Truncate(nameText, nameW, m.theme.ellipsis())
	}
	nameStyle := m.theme.style(m.theme.text)
	if selected {
		nameStyle = m.theme.bold(m.theme.text)
	}
	if f := m.flashAmount(r); f > 0 {
		if c := m.theme.flashColor(status); c != nil {
			nameStyle = nameStyle.Background(c)
		}
	}
	name := nameStyle.Render(padPlain(nameText, nameW))
	statusText := status.String()
	statusCell := m.theme.style(m.theme.statusColor(status)).Render(padPlain(statusText, statusW))

	parts := []string{cursor, glyph}
	if !compact {
		const targetW = 8
		parts = append(parts, m.theme.style(m.theme.muted).Render(padPlain(textutil.Clean(r.job.TargetLabel), targetW)))
	}
	parts = append(parts, name, statusCell)

	if !compact && w >= 72 {
		const targetW = 8
		metaW := max(0, w-2-2-(targetW+1)-nameW-statusW-2-3)
		meta := m.statusMeta(r)
		parts = append(parts, m.theme.style(m.theme.muted).Render(padPlain(meta, metaW)))
	}
	if w >= 90 && r.result.SourceURL != "" {
		link := "↗"
		if m.opts.ASCII {
			link = "open"
		}
		parts = append(parts, m.theme.style(m.theme.dim).Render(link))
	}
	return fit(strings.Join(parts, " "), w)
}

func (m *model) statusMeta(r *row) string {
	switch r.status {
	case check.StatusQueued:
		return "—"
	case check.StatusChecking:
		if r.job.Category == check.CategoryDomains {
			return "dns → rdap"
		}
		return "requesting"
	}
	var parts []string
	summary := textutil.Clean(r.result.Summary)
	if r.status == check.StatusAvailable {
		switch summary {
		case "no record", "no registration record", "no WHOIS record":
			summary = ""
		}
	}
	if summary != "" {
		parts = append(parts, summary)
	}
	if r.status == check.StatusAvailable && r.result.Confidence == check.Likely {
		parts = append(parts, "likely")
	}
	if r.result.Cached {
		parts = append(parts, "cached")
	} else if r.result.Latency > 0 {
		parts = append(parts, r.result.Latency.Round(time.Millisecond).String())
	}
	if len(parts) == 0 && r.result.Err != nil {
		parts = append(parts, textutil.Clean(check.ShortError(r.result.Err)))
	}
	return strings.Join(parts, " · ")
}

func (m *model) footerLines(w int) []string {
	var lines []string
	if m.filterOn {
		lines = append(lines, m.filter.View())
	}
	if m.toast != "" {
		lines = append(lines, m.theme.style(m.theme.accent).Render(m.toast))
	}
	hint := "↑↓ move   ⏎ details   / filter   a available   s sort   r rerun   ? help   q quit"
	if w < 90 {
		hint = "↑↓ move  ⏎ details  / filter  a avail  e export  esc new  q quit"
	}
	if m.phase == phaseDone && w >= 90 {
		hint = "↑↓ move   ⏎ details   / filter   a available   e export   esc new search   q quit"
	}
	if m.opts.ASCII {
		hint = "up/down move   enter details   / filter   a available   s sort   r rerun   ? help   q quit"
	}
	lines = append(lines, m.rule(w))
	lines = append(lines, m.theme.style(m.theme.dim).Render(hint))
	return lines
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

func truncateText(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
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

func center(s string, w int) string {
	pad := (w - lipgloss.Width(s)) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
