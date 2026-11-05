package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/textutil"
)

type detailState struct {
	row int
	vp  viewport.Model
}

func (m *model) detailSize() (int, int) {
	w := min(80, m.width-4)
	h := min(23, m.height-4)
	if w < 36 {
		w = max(30, m.width-2)
	}
	if h < 10 {
		h = max(3, m.height-2)
	}
	if h > m.height {
		h = max(3, m.height-2)
	}
	if w > m.width {
		w = max(3, m.width-2)
	}
	return w, h
}

func (m *model) openDetail(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.rows) {
		return nil
	}
	m.detail = &detailState{row: idx}
	m.reopenDetail()
	m.anim.detailPos = 6
	m.anim.detailVel = 0
	m.anim.detailTarget = 0
	m.now = time.Now()
	return m.tickIfNeeded()
}

func (m *model) reopenDetail() {
	if m.detail == nil || m.detail.row < 0 || m.detail.row >= len(m.rows) {
		return
	}
	w, h := m.detailSize()
	contentW := max(10, w-2)
	contentH := max(3, h-3) // border + footer
	oldOffset := 0
	if m.detail.vp.Width() > 0 {
		oldOffset = m.detail.vp.YOffset()
	}
	vp := viewport.New(viewport.WithWidth(contentW), viewport.WithHeight(contentH))
	vp.SoftWrap = true
	vp.Style = lipgloss.NewStyle()
	vp.SetContent(m.detailContent(m.rows[m.detail.row], contentW))
	vp.SetYOffset(oldOffset)
	m.detail.vp = vp
}

func (m *model) detailContent(r *row, width int) string {
	var b strings.Builder
	status := r.status
	if status == check.StatusQueued && r.job.Err != nil {
		status = check.StatusSkipped
	}
	glyph := m.theme.glyph(status, "")
	color := m.theme.statusColor(status)
	confidence := r.result.Confidence
	if !status.IsFinal() {
		confidence = check.Low
	}
	head := m.theme.bold(color).Render(glyph+" "+status.String()) +
		m.theme.style(m.theme.muted).Render("  ·  "+confidence.String())
	if r.result.Cached {
		head += m.theme.style(m.theme.amber).Render("  ·  cached")
	}
	if r.result.Latency > 0 {
		head += m.theme.style(m.theme.muted).Render(fmt.Sprintf("  ·  %s", r.result.Latency.Round(time.Millisecond)))
	}
	b.WriteString(head)
	b.WriteString("\n\n")

	if summary := textutil.Clean(r.result.Summary); summary != "" {
		b.WriteString(m.theme.style(m.theme.text).Render(wrapText(summary, width)))
		b.WriteString("\n\n")
	}

	fields := r.result.Fields
	if len(fields) == 0 {
		b.WriteString(m.theme.style(m.theme.muted).Render("No metadata returned."))
		b.WriteString("\n")
	}
	for _, f := range fields {
		if f.Value == "" {
			continue
		}
		key := padPlain(textutil.Clean(f.Key), 12)
		value := wrapText(textutil.Clean(f.Value), max(10, width-14))
		lines := strings.Split(value, "\n")
		b.WriteString(m.theme.style(m.theme.muted).Render(key))
		b.WriteString(m.theme.style(m.theme.text).Render(lines[0]))
		b.WriteString("\n")
		for _, line := range lines[1:] {
			b.WriteString(strings.Repeat(" ", 12))
			b.WriteString(m.theme.style(m.theme.text).Render(line))
			b.WriteString("\n")
		}
	}

	if r.job.Category == check.CategoryDomains {
		b.WriteString("\n")
		note := "Domain availability only means no RDAP registration record was found. Reserved and premium names can still be unavailable or cost more; confirm with a registrar."
		if status == check.StatusTaken {
			switch {
			case r.result.Field("registrar") != "":
				note = "This domain has a registration record. The registrar and dates above come from RDAP; DNS delegation is a faster but less detailed signal."
			case r.result.Field("dns") != "":
				note = "This result comes from DNS delegation: the domain is registered. Registrar and dates are only available from RDAP or WHOIS."
			}
		}
		b.WriteString(m.theme.style(m.theme.muted).Render(wrapText(note, width)))
		b.WriteString("\n")
	}

	if r.result.Err != nil {
		b.WriteString("\n")
		b.WriteString(m.theme.style(m.theme.red).Render("error: " + wrapText(textutil.Clean(r.result.Err.Error()), max(10, width-7))))
		b.WriteString("\n")
	}
	if r.result.SourceURL != "" {
		b.WriteString("\n")
		b.WriteString(m.theme.style(m.theme.dim).Render(wrapText(textutil.Clean(r.result.SourceURL), width)))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *model) detailPanel() string {
	if m.detail == nil || m.detail.row < 0 || m.detail.row >= len(m.rows) {
		return ""
	}
	r := m.rows[m.detail.row]
	w, h := m.detailSize()
	title := textutil.Clean(r.job.Label) + " · " + textutil.Clean(r.job.TargetLabel)
	body := strings.Split(m.detail.vp.View(), "\n")
	footer := m.theme.style(m.theme.dim).Render("↑↓ scroll   o open   c copy   esc close")
	body = append(body, footer)
	return m.frame(title, body, w, h)
}

func (m *model) helpPanel() string {
	w := min(72, m.width-4)
	h := min(24, m.height-4)
	if w < 40 {
		w = max(32, m.width-2)
	}
	if h < 12 {
		h = max(3, m.height-2)
	}
	if h > m.height {
		h = max(3, m.height-2)
	}
	if w > m.width {
		w = max(3, m.width-2)
	}

	type row struct{ key, desc string }
	groups := []struct {
		title string
		rows  []row
	}{
		{"MOVE", []row{
			{"↑/k, ↓/j", "move between rows"},
			{"g / G", "jump to top or bottom"},
			{"pgup/pgdn", "page up or down"},
			{"⏎", "open details"},
		}},
		{"FILTER", []row{
			{"/", "filter by name, target or status"},
			{"a", "toggle only-available"},
			{"s", "cycle sort: group, status, latency"},
			{"esc", "close filter / cancel run"},
		}},
		{"ACTIONS", []row{
			{"c", "copy name or source URL"},
			{"o", "open source URL"},
			{"r", "rerun this name"},
			{"e / E", "export JSON / Markdown"},
			{"?", "close this help"},
			{"q", "quit"},
		}},
	}
	var lines []string
	for _, g := range groups {
		lines = append(lines, m.theme.bold(m.theme.accent).Render(g.title))
		for _, r := range g.rows {
			lines = append(lines,
				"  "+m.theme.style(m.theme.text).Render(padPlain(r.key, 12))+
					m.theme.style(m.theme.muted).Render(r.desc))
		}
		lines = append(lines, "")
	}
	return m.frame("help", lines, w, h)
}

func (m *model) overlay(base, panel string) string {
	w, h := m.width, m.height
	if panel == "" {
		return base
	}
	pw := lipgloss.Width(panel)
	ph := lipgloss.Height(panel)
	x := max(0, (w-pw)/2)
	y := max(0, (h-ph)/2)
	if m.detail != nil {
		y += int(m.anim.detailPos)
	}
	if x+pw > w {
		x = max(0, w-pw)
	}
	if y+ph > h {
		y = max(0, h-ph)
	}
	if y < 0 {
		y = 0
	}
	baseLayer := lipgloss.NewLayer(base)
	panelLayer := lipgloss.NewLayer(panel).X(x).Y(y).Z(1)
	out := lipgloss.NewCompositor(baseLayer, panelLayer).Render()
	lines := strings.Split(out, "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = fit(lines[i], w)
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	var line strings.Builder
	for _, w := range words {
		if line.Len() == 0 {
			line.WriteString(w)
			continue
		}
		if line.Len()+1+len(w) > width {
			lines = append(lines, line.String())
			line.Reset()
			line.WriteString(w)
			continue
		}
		line.WriteString(" ")
		line.WriteString(w)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}
