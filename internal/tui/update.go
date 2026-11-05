package tui

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/engine"
	"github.com/yufeiminds/nameary/internal/textutil"
)

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case tea.BackgroundColorMsg:
		m.theme.setDark(msg.IsDark())
		m.spinner.Style = m.theme.style(m.theme.accent)
		m.applyInputStyles(&m.input)
		m.applyInputStyles(&m.filter)
		return m, nil

	case frameMsg:
		return m, m.onFrame(time.Time(msg))

	case eventMsg:
		if msg.runID != m.runID {
			return m, nil
		}
		return m, m.handleEvent(msg.ev)

	case streamDoneMsg:
		if msg.runID != m.runID {
			return m, nil
		}
		return m, m.finish()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.phase == phaseRunning && m.hasPending() {
			return m, cmd
		}
		return m, nil

	case progress.FrameMsg:
		var cmd tea.Cmd
		m.prog, cmd = m.prog.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.PasteMsg:
		if m.phase == phaseInput {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.inputErr = nil
			return m, cmd
		}
		if m.filterOn {
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			m.rebuildItems()
			return m, cmd
		}
	}

	return m, nil
}

func (m *model) handleEvent(ev engine.Event) tea.Cmd {
	idx, ok := m.byID[ev.Job.ID]
	if !ok {
		return waitEvent(m.events, m.runID)
	}
	r := m.rows[idx]
	switch ev.Kind {
	case engine.EventStarted:
		if !r.status.IsFinal() {
			r.status = check.StatusChecking
			r.startedAt = time.Now()
		}
	case engine.EventSettled:
		wasFinal := r.status.IsFinal()
		r.result = ev.Result
		r.status = ev.Result.Status
		if r.status == check.StatusQueued {
			r.status = check.StatusUnknown
		}
		r.settledAt = time.Now()
		if !wasFinal {
			m.settled++
			m.counts[r.status]++
		}
	}
	m.now = time.Now()
	if m.detail != nil && m.detail.row == idx {
		m.reopenDetail()
	}
	if m.filterOn || m.onlyAvailable || m.sortMode != sortGroup {
		m.rebuildItems()
	}
	pct := m.progressRatio()
	return tea.Batch(
		waitEvent(m.events, m.runID),
		m.prog.SetPercent(pct),
		m.tickIfNeeded(),
	)
}

func (m *model) finish() tea.Cmd {
	now := time.Now()
	for _, r := range m.rows {
		if r.status.IsFinal() {
			continue
		}
		r.status = check.StatusCanceled
		r.result.Status = check.StatusCanceled
		r.result.Confidence = check.Low
		r.result.Summary = "canceled"
		r.settledAt = now
		m.settled++
		m.counts[check.StatusCanceled]++
	}
	m.phase = phaseDone
	m.finished = now
	m.now = now
	m.anim.summaryStart = now
	if m.filterOn || m.onlyAvailable || m.sortMode != sortGroup {
		m.rebuildItems()
	}
	if m.opts.Celebrate && m.counts[check.StatusTaken] == 0 && m.counts[check.StatusAvailable] > 0 {
		m.toast = "all clear · every checked name is available"
		m.toastUntil = now.Add(5 * time.Second)
	}
	return tea.Batch(m.prog.SetPercent(1), m.tickIfNeeded())
}

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		m.closeRun()
		return m, tea.Quit
	}

	if m.helpOpen {
		switch k {
		case "esc", "?", "q", "enter":
			m.helpOpen = false
		}
		return m, nil
	}

	if m.detail != nil {
		switch k {
		case "esc":
			m.detail = nil
			m.anim.detailPos = 0
			return m, nil
		case "up", "k":
			m.detail.vp.ScrollUp(1)
		case "down", "j":
			m.detail.vp.ScrollDown(1)
		case "pgup":
			m.detail.vp.PageUp()
		case "pgdown":
			m.detail.vp.PageDown()
		case "g", "home":
			m.detail.vp.GotoTop()
		case "G", "end":
			m.detail.vp.GotoBottom()
		case "o":
			return m, m.openCurrent()
		case "c":
			return m, m.copyCurrent()
		case "?":
			m.helpOpen = true
		case "q":
			m.closeRun()
			return m, tea.Quit
		}
		return m, nil
	}

	if m.filterOn {
		switch k {
		case "esc":
			m.filterOn = false
			m.filter.SetValue("")
			m.filter.Blur()
			m.rebuildItems()
			return m, nil
		case "enter":
			m.filterOn = false
			m.filter.Blur()
			m.rebuildItems()
			return m, nil
		case "up", "down", "pgup", "pgdown":
			// Fall through to navigation once the filter is applied.
			m.filterOn = false
			m.filter.Blur()
			m.rebuildItems()
			return m.handleKey(msg)
		}
		old := m.filter.Value()
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		if old != m.filter.Value() {
			m.rebuildItems()
		}
		return m, cmd
	}

	if m.phase == phaseInput {
		switch k {
		case "?":
			m.helpOpen = true
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				m.inputErr = fmt.Errorf("type a name first")
				return m, nil
			}
			cmd := m.planAndStart(name)
			return m, cmd
		case "esc":
			m.input.Reset()
			m.inputErr = nil
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.inputErr = nil
		return m, cmd
	}

	switch k {
	case "q":
		m.closeRun()
		return m, tea.Quit
	case "esc":
		m.focusInput(m.name)
		return m, m.tickIfNeeded()
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup":
		m.page(-1)
	case "pgdown":
		m.page(1)
	case "g", "home":
		m.moveTo(true)
	case "G", "end":
		m.moveTo(false)
	case "enter":
		if r := m.currentRow(); r != nil {
			idx := m.items[m.cursor].row
			return m, m.openDetail(idx)
		}
	case "/":
		m.filterOn = true
		m.filter.SetValue("")
		cmd := m.filter.Focus()
		return m, cmd
	case "a":
		m.onlyAvailable = !m.onlyAvailable
		m.rebuildItems()
		if m.onlyAvailable {
			return m, m.setToast("showing available only")
		}
		return m, m.setToast("showing all results")
	case "s":
		m.sortMode = (m.sortMode + 1) % 3
		m.rebuildItems()
		names := []string{"group order", "status", "latency"}
		return m, m.setToast("sort: " + names[m.sortMode])
	case "r", "R":
		if m.name == "" {
			return m, nil
		}
		return m, m.planAndStart(m.name)
	case "e":
		return m, m.exportJSON()
	case "E":
		return m, m.exportMarkdown()
	case "c":
		return m, m.copyCurrent()
	case "o":
		return m, m.openCurrent()
	case "?":
		m.helpOpen = true
		return m, nil
	case "tab":
		m.nextGroup()
	}
	return m, nil
}

func (m *model) nextGroup() {
	if len(m.items) == 0 {
		return
	}
	start := m.cursor + 1
	for i := 0; i < len(m.items); i++ {
		idx := (start + i) % len(m.items)
		if m.items[idx].kind == itemHeader {
			for j := idx + 1; j < len(m.items); j++ {
				if m.items[j].kind == itemRow {
					m.cursor = j
					m.clampOffset()
					return
				}
			}
			for j := 0; j < idx; j++ {
				if m.items[j].kind == itemRow {
					m.cursor = j
					m.clampOffset()
					return
				}
			}
		}
	}
}

func (m *model) copyCurrent() tea.Cmd {
	r := m.currentRow()
	if r == nil {
		return nil
	}
	text := textutil.Clean(r.job.Label)
	if u, ok := safeExternalURL(r.result.SourceURL); ok {
		text = u
	}
	cmd := m.setToast("copied " + text)
	return tea.Batch(tea.SetClipboard(text), cmd)
}

func (m *model) openCurrent() tea.Cmd {
	r := m.currentRow()
	if r == nil || r.result.SourceURL == "" {
		return m.setToast("no source URL for this result")
	}
	u, ok := safeExternalURL(r.result.SourceURL)
	if !ok {
		return m.setToast("refusing to open a non-http(s) URL")
	}
	return tea.Batch(openURL(u), m.setToast("opening browser"))
}

// safeExternalURL only permits ordinary web URLs. This prevents a malicious
// registry response from launching file:// or custom-protocol handlers.
func safeExternalURL(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	return u.String(), true
}

func openURL(u string) tea.Cmd {
	return func() tea.Msg {
		if _, ok := safeExternalURL(u); !ok {
			return nil
		}
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			// #nosec G204 -- u is restricted to http(s) URLs and exec.Command does not invoke a shell.
			cmd = exec.Command("open", u)
		case "windows":
			// #nosec G204 -- u is restricted to http(s) URLs and exec.Command does not invoke a shell.
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
		default:
			// #nosec G204 -- u is restricted to http(s) URLs and exec.Command does not invoke a shell.
			cmd = exec.Command("xdg-open", u)
		}
		if err := cmd.Start(); err == nil {
			go func() { _ = cmd.Wait() }()
		}
		return nil
	}
}
