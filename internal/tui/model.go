package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/engine"
	"github.com/yufeiminds/nameary/internal/textutil"
)

type phase int

const (
	phaseInput phase = iota
	phaseRunning
	phaseDone
)

type sortMode int

const (
	sortGroup sortMode = iota
	sortStatus
	sortLatency
)

// Options configures the TUI.
type Options struct {
	InitialName   string
	Targets       []string
	TLDs          []string
	Engine        *engine.Engine
	Plan          func(string) ([]engine.Job, error)
	ReducedMotion bool
	ASCII         bool
	NoColor       bool
	Celebrate     bool
}

type row struct {
	job       engine.Job
	status    check.Status
	result    check.Result
	startedAt time.Time
	settledAt time.Time
}

type itemKind int

const (
	itemHeader itemKind = iota
	itemRow
)

type item struct {
	kind    itemKind
	group   check.Category
	row     int
	total   int
	settled int
}

type model struct {
	ctx  context.Context
	opts Options

	width, height int
	phase         phase
	name          string

	input    textinput.Model
	inputErr error
	filter   textinput.Model
	filterOn bool

	rows  []*row
	byID  map[string]int
	items []item

	cursor int
	offset int

	onlyAvailable bool
	sortMode      sortMode

	runID    uint64
	events   <-chan engine.Event
	cancel   context.CancelFunc
	started  time.Time
	finished time.Time

	settled int
	counts  map[check.Status]int

	spinner spinner.Model
	prog    progress.Model
	theme   theme

	detail   *detailState
	helpOpen bool

	toast      string
	toastUntil time.Time
	now        time.Time

	anim animState
}

type eventMsg struct {
	runID uint64
	ev    engine.Event
}

type streamDoneMsg struct {
	runID uint64
}

func newModel(ctx context.Context, opts Options) *model {
	m := &model{
		ctx:    ctx,
		opts:   opts,
		width:  80,
		height: 24,
		counts: map[check.Status]int{},
		theme:  newTheme(opts.NoColor, opts.ASCII),
		anim:   newAnim(),
		now:    time.Now(),
	}

	m.input = textinput.New()
	m.input.Prompt = "❯ "
	if opts.ASCII {
		m.input.Prompt = "> "
	}
	m.input.Placeholder = "acme"
	m.input.CharLimit = 256
	m.input.SetWidth(40)
	m.input.Focus()
	m.applyInputStyles(&m.input)

	m.filter = textinput.New()
	m.filter.Prompt = "filter: "
	m.filter.Placeholder = "name, target, status"
	m.filter.CharLimit = 80
	m.filter.SetWidth(40)
	m.applyInputStyles(&m.filter)

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(m.theme.style(m.theme.accent)),
	)
	m.prog = progress.New(
		progress.WithDefaultBlend(),
		progress.WithoutPercentage(),
		progress.WithFillCharacters('█', '░'),
		progress.WithWidth(40),
		progress.WithSpringOptions(12, 1),
	)

	if opts.InitialName != "" {
		m.input.SetValue(opts.InitialName)
		if cmd := m.planAndStart(opts.InitialName); cmd == nil && m.phase != phaseRunning {
			m.phase = phaseInput
			m.input.Focus()
			m.anim.logoStart = time.Now()
		}
	} else {
		m.phase = phaseInput
		m.anim.logoStart = time.Now()
	}
	m.resize()
	return m
}

func (m *model) applyInputStyles(in *textinput.Model) {
	styles := in.Styles()
	styles.Focused.Prompt = m.theme.style(m.theme.accent)
	styles.Focused.Text = m.theme.style(m.theme.text)
	styles.Focused.Placeholder = m.theme.style(m.theme.dim)
	styles.Blurred.Prompt = m.theme.style(m.theme.dim)
	styles.Blurred.Text = m.theme.style(m.theme.muted)
	styles.Blurred.Placeholder = m.theme.style(m.theme.dim)
	in.SetStyles(styles)
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor}
	if m.phase == phaseInput {
		if !m.opts.ReducedMotion {
			m.anim.logoStart = time.Now()
			cmds = append(cmds, m.tickIfNeeded())
		}
		cmds = append(cmds, m.input.Focus())
		return tea.Batch(cmds...)
	}
	cmds = append(cmds,
		waitEvent(m.events, m.runID),
		m.spinner.Tick,
		m.tickIfNeeded(),
	)
	return tea.Batch(cmds...)
}

func (m *model) planAndStart(name string) tea.Cmd {
	name = strings.TrimSpace(name)
	if m.opts.Engine == nil || m.opts.Plan == nil {
		m.inputErr = fmt.Errorf("engine is not configured")
		return nil
	}
	jobs, err := m.opts.Plan(name)
	if err != nil {
		m.inputErr = err
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}

	m.name = textutil.Clean(name)
	m.rows = buildRows(jobs)
	m.byID = make(map[string]int, len(m.rows))
	for i, r := range m.rows {
		m.byID[r.job.ID] = i
	}
	m.counts = map[check.Status]int{check.StatusQueued: len(m.rows)}
	m.settled = 0
	m.runID++
	runCtx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.events = m.opts.Engine.Run(runCtx, m.runID, jobs)
	m.phase = phaseRunning
	m.started = time.Now()
	m.finished = time.Time{}
	m.detail = nil
	m.helpOpen = false
	m.filterOn = false
	m.filter.SetValue("")
	m.filter.Blur()
	m.onlyAvailable = false
	m.sortMode = sortGroup
	m.input.Blur()
	m.inputErr = nil
	m.toast = ""
	m.toastUntil = time.Time{}
	m.now = time.Now()
	m.anim.logoStart = m.now
	m.rebuildItems()
	m.cursor = firstRow(m.items)
	m.offset = 0
	m.prog.SetPercent(0)

	return tea.Batch(
		waitEvent(m.events, m.runID),
		m.spinner.Tick,
		m.prog.SetPercent(0),
		m.tickIfNeeded(),
	)
}

func waitEvent(ch <-chan engine.Event, runID uint64) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return streamDoneMsg{runID: runID}
		}
		return eventMsg{runID: runID, ev: ev}
	}
}

func buildRows(jobs []engine.Job) []*row {
	rows := make([]*row, 0, len(jobs))
	for _, j := range jobs {
		rows = append(rows, &row{job: j, status: check.StatusQueued})
	}
	return rows
}

func (m *model) resize() {
	if m.width <= 0 {
		m.width = 80
	}
	if m.height <= 0 {
		m.height = 24
	}
	m.input.SetWidth(max(20, min(50, m.width-20)))
	m.filter.SetWidth(max(20, min(56, m.width-20)))
	m.prog.SetWidth(max(10, m.width-10))
	if m.detail != nil {
		m.reopenDetail()
	}
	m.clampOffset()
}

func (m *model) closeRun() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func (m *model) setToast(text string) tea.Cmd {
	m.toast = textutil.Clean(text)
	m.toastUntil = time.Now().Add(2 * time.Second)
	m.now = time.Now()
	return m.tickIfNeeded()
}

func (m *model) progressRatio() float64 {
	if len(m.rows) == 0 {
		return 0
	}
	return float64(m.settled) / float64(len(m.rows))
}

func (m *model) currentRow() *row {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	it := m.items[m.cursor]
	if it.kind != itemRow {
		return nil
	}
	return m.rows[it.row]
}

func firstRow(items []item) int {
	for i, it := range items {
		if it.kind == itemRow {
			return i
		}
	}
	return 0
}

func (m *model) focusInput(prefill string) {
	m.closeRun()
	m.runID++
	m.phase = phaseInput
	m.filterOn = false
	m.filter.SetValue("")
	m.filter.Blur()
	m.input.SetValue(textutil.Clean(prefill))
	m.input.Focus()
	m.inputErr = nil
	m.detail = nil
	m.helpOpen = false
	m.anim.logoStart = time.Now()
}
