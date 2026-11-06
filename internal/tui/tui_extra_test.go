package tui

import (
	"context"
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/engine"
)

func press(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		r := []rune(s)[0]
		return tea.KeyPressMsg{Code: r, Text: s}
	}
}

func testModelWithRows() *model {
	m := testModel()
	m.byID = map[string]int{}
	for i, r := range m.rows {
		m.byID[r.job.ID] = i
	}
	return m
}

func TestUpdateMessages(t *testing.T) {
	m := testModelWithRows()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if next != m || m.width != 100 || m.height != 40 {
		t.Fatalf("window size: %dx%d", m.width, m.height)
	}

	_, _ = m.Update(tea.BackgroundColorMsg{Color: color.Black})
	if !m.theme.dark {
		t.Fatal("dark background not applied")
	}
	_, _ = m.Update(tea.BackgroundColorMsg{Color: color.White})
	if m.theme.dark {
		t.Fatal("light background not applied")
	}

	_, _ = m.Update(frameMsg(time.Now()))
	if m.anim.frame == 0 {
		t.Fatal("frame not counted")
	}

	_, _ = m.Update(eventMsg{runID: m.runID + 1, ev: engine.Event{}})
	_, _ = m.Update(streamDoneMsg{runID: m.runID + 1})
	_, _ = m.Update(spinner.TickMsg{})
	_, _ = m.Update(progress.FrameMsg{})
	_, _ = m.Update(tea.PasteMsg{Content: "x"})
	_, _ = m.Update(struct{}{})
}

func TestHandleEvent(t *testing.T) {
	m := testModelWithRows()
	m.rows[0].status = check.StatusChecking
	m.rows[1].status = check.StatusQueued
	m.settled = 0
	m.counts = map[check.Status]int{}

	cmd := m.handleEvent(engine.Event{Kind: engine.EventStarted, Job: m.rows[1].job})
	if cmd == nil || m.rows[1].status != check.StatusChecking {
		t.Fatalf("started not applied: %v", m.rows[1].status)
	}

	m.handleEvent(engine.Event{Kind: engine.EventSettled, Job: m.rows[0].job, Result: check.Result{Status: check.StatusTaken, Confidence: check.Certain}})
	if m.settled != 1 || m.counts[check.StatusTaken] != 1 {
		t.Fatalf("settled=%d counts=%v", m.settled, m.counts)
	}
	// A duplicate settle must not double-count.
	m.handleEvent(engine.Event{Kind: engine.EventSettled, Job: m.rows[0].job, Result: check.Result{Status: check.StatusTaken}})
	if m.settled != 1 {
		t.Fatalf("duplicate settle counted: %d", m.settled)
	}

	m.rows[1].status = check.StatusQueued
	m.handleEvent(engine.Event{Kind: engine.EventSettled, Job: m.rows[1].job, Result: check.Result{Status: check.StatusQueued}})
	if m.rows[1].status != check.StatusUnknown {
		t.Fatalf("queued settle should become unknown: %v", m.rows[1].status)
	}

	// Unknown job simply waits for the next event.
	if cmd := m.handleEvent(engine.Event{Job: engine.Job{ID: "nope"}}); cmd == nil {
		t.Fatal("expected wait command")
	}

	_ = m.openDetail(0)
	m.handleEvent(engine.Event{Kind: engine.EventSettled, Job: m.rows[0].job, Result: check.Result{Status: check.StatusAvailable}})
	m.filterOn = true
	m.handleEvent(engine.Event{Kind: engine.EventSettled, Job: m.rows[0].job, Result: check.Result{Status: check.StatusAvailable}})
}

func TestFinish(t *testing.T) {
	m := testModelWithRows()
	m.phase = phaseRunning
	m.rows[0].status = check.StatusTaken
	m.settled = 1
	m.counts = map[check.Status]int{check.StatusTaken: 1}
	m.rows[1].status = check.StatusChecking
	m.opts.Celebrate = true
	m.opts.Celebrate = false
	cmd := m.finish()
	if cmd == nil || m.phase != phaseDone {
		t.Fatalf("phase=%v cmd=%v", m.phase, cmd)
	}
	if m.rows[1].status != check.StatusCanceled || m.counts[check.StatusCanceled] != 1 {
		t.Fatalf("canceled row=%v counts=%v", m.rows[1].status, m.counts)
	}

	m2 := testModelWithRows()
	m2.phase = phaseRunning
	m2.rows[1].status = check.StatusAvailable
	m2.counts = map[check.Status]int{check.StatusAvailable: 1}
	m2.opts.Celebrate = true
	m2.finish()
	if m2.toast == "" {
		t.Fatal("expected celebration toast")
	}
}

func TestUpdateKeyRouting(t *testing.T) {
	m := testModelWithRows()

	// Help overlay swallows every key.
	m.helpOpen = true
	for _, k := range []string{"esc", "?", "q", "enter", "x"} {
		_, _ = m.handleKey(press(k))
	}
	if m.helpOpen {
		t.Fatal("help should close")
	}

	// Detail overlay keys.
	_ = m.openDetail(0)
	for _, k := range []string{"up", "down", "pgup", "pgdown", "g", "G", "home", "end", "o", "c", "?", "x"} {
		_, _ = m.handleKey(press(k))
	}
	if !m.helpOpen {
		t.Fatal("help should open from detail")
	}
	m.helpOpen = false
	_, _ = m.handleKey(press("esc"))
	if m.detail != nil {
		t.Fatal("detail should close")
	}

	// Filter mode.
	m.filterOn = true
	m.filter.Focus()
	m.filter.SetValue("")
	_, _ = m.handleKey(press("n"))
	if !strings.Contains(m.filter.Value(), "n") {
		t.Fatalf("filter=%q", m.filter.Value())
	}
	_, _ = m.handleKey(press("up"))
	if m.filterOn {
		t.Fatal("up should leave filter and navigate")
	}
	m.filterOn = true
	m.filter.Focus()
	_, _ = m.handleKey(press("enter"))
	if m.filterOn {
		t.Fatal("enter should close filter")
	}
	m.filterOn = true
	m.filter.Focus()
	_, _ = m.handleKey(press("esc"))
	if m.filterOn {
		t.Fatal("esc should close filter")
	}

	// Input phase.
	m.phase = phaseInput
	_, _ = m.handleKey(press("?"))
	if !m.helpOpen {
		t.Fatal("help from input")
	}
	m.helpOpen = false
	_, _ = m.handleKey(press("esc"))
	_, _ = m.handleKey(press("x"))
	_, _ = m.handleKey(press("enter"))
	if m.inputErr == nil {
		t.Fatal("expected empty input error")
	}
	m.opts.Engine = engine.New(engine.Options{Concurrency: 1})
	m.opts.Plan = func(string) ([]engine.Job, error) { return nil, errors.New("plan failed") }
	_, _ = m.handleKey(press("enter"))
	if m.inputErr == nil || !strings.Contains(m.inputErr.Error(), "plan failed") {
		t.Fatalf("inputErr=%v", m.inputErr)
	}

	// ctrl+c always quits.
	if _, cmd := m.handleKey(press("ctrl+c")); cmd == nil {
		t.Fatal("ctrl+c should quit")
	}
}

func TestDoneKeys(t *testing.T) {
	m := testModelWithRows()
	m.width, m.height = 100, 40
	m.rebuildItems()

	for _, k := range []string{"down", "j", "up", "k", "pgdown", "pgup", "g", "home", "G", "end", "/", "a", "s", "tab", "?", "x"} {
		_, _ = m.handleKey(press(k))
	}
	m.helpOpen = false
	m.filterOn = false
	m.filter.Blur()
	m.filter.SetValue("")
	m.onlyAvailable = false
	m.rebuildItems()
	m.cursor = firstRow(m.items)
	if _, cmd := m.handleKey(press("enter")); cmd == nil || m.detail == nil {
		t.Fatal("enter should open detail")
	}
	m.detail = nil

	m.onlyAvailable = false
	_, _ = m.handleKey(press("a"))
	if !m.onlyAvailable {
		t.Fatal("only-available not toggled on")
	}
	_, _ = m.handleKey(press("a"))
	if m.onlyAvailable {
		t.Fatal("only-available not toggled off")
	}

	before := m.sortMode
	_, _ = m.handleKey(press("s"))
	if m.sortMode == before {
		t.Fatal("sort did not advance")
	}

	m.opts.Plan = func(string) ([]engine.Job, error) { return nil, errors.New("no") }
	_, _ = m.handleKey(press("r"))
	if m.inputErr == nil {
		t.Fatal("rerun should surface plan error")
	}

	if _, cmd := m.handleKey(press("q")); cmd == nil {
		t.Fatal("q should quit")
	}
	m.handleKey(press("esc"))
	if m.phase != phaseInput {
		t.Fatalf("esc should focus input, phase=%v", m.phase)
	}
}

func TestDetailKeysWithOpenAndCopy(t *testing.T) {
	m := testModelWithRows()
	m.rows[0].result.SourceURL = "https://example.test/acme"
	_ = m.openDetail(0)
	if _, cmd := m.handleKey(press("o")); cmd == nil {
		t.Fatal("open should return a command")
	}
	if _, cmd := m.handleKey(press("c")); cmd == nil {
		t.Fatal("copy should return a command")
	}
	if _, cmd := m.handleKey(press("q")); cmd == nil {
		t.Fatal("q in detail should quit")
	}
	m.handleKey(press("?"))
	_, _ = m.handleKey(press("q"))
	if m.helpOpen {
		t.Fatal("q in help should close help")
	}
}

func TestPlanAndStartBranches(t *testing.T) {
	m := newModel(context.Background(), Options{NoColor: true, ASCII: true})
	if cmd := m.planAndStart("acme"); cmd != nil || m.inputErr == nil {
		t.Fatal("nil engine should error")
	}

	m.opts.Plan = func(string) ([]engine.Job, error) { return nil, errors.New("boom") }
	if cmd := m.planAndStart("acme"); cmd != nil || m.inputErr == nil {
		t.Fatal("plan error should surface")
	}

	m.opts.Engine = engine.New(engine.Options{Concurrency: 1})
	m.opts.Plan = func(string) ([]engine.Job, error) {
		return []engine.Job{{ID: "1", Target: "fake", Raw: "acme", Label: "acme"}}, nil
	}
	cmd := m.planAndStart("acme")
	if cmd == nil || len(m.rows) != 1 || len(m.byID) != 1 || m.phase != phaseRunning {
		t.Fatalf("rows=%d byID=%d phase=%v", len(m.rows), len(m.byID), m.phase)
	}
	m.closeRun()

	// InitialName triggers the same path from newModel.
	m2 := newModel(context.Background(), Options{
		NoColor: true, ASCII: true, InitialName: "acme",
		Engine: engine.New(engine.Options{Concurrency: 1}),
		Plan: func(string) ([]engine.Job, error) {
			return []engine.Job{{ID: "1", Target: "fake", Raw: "acme"}}, nil
		},
	})
	if m2.phase != phaseRunning {
		t.Fatalf("phase=%v", m2.phase)
	}
	m2.closeRun()

	m3 := newModel(context.Background(), Options{
		NoColor: true, ASCII: true, InitialName: "acme",
		Plan: func(string) ([]engine.Job, error) { return nil, errors.New("x") },
	})
	if m3.phase != phaseInput || m3.inputErr == nil {
		t.Fatalf("phase=%v inputErr=%v", m3.phase, m3.inputErr)
	}
}

func TestNextGroupAndNavigation(t *testing.T) {
	m := testModelWithRows()
	m.rebuildItems()
	m.nextGroup()
	if m.items[m.cursor].kind != itemRow {
		t.Fatal("cursor should land on a row")
	}
	m.items = nil
	m.nextGroup()
	m.move(1)
	m.moveTo(true)
	m.moveTo(false)
	m.page(1)
}

func TestCopyAndOpenCurrent(t *testing.T) {
	m := testModelWithRows()
	m.cursor = firstRow(m.items)
	m.rows[0].result.SourceURL = ""
	if cmd := m.copyCurrent(); cmd == nil || !strings.Contains(m.toast, "copied") {
		t.Fatalf("toast=%q", m.toast)
	}
	if cmd := m.openCurrent(); cmd == nil || !strings.Contains(m.toast, "no source URL") {
		t.Fatalf("toast=%q", m.toast)
	}
	m.rows[0].result.SourceURL = "file:///etc/passwd"
	if cmd := m.openCurrent(); cmd == nil || !strings.Contains(m.toast, "refusing") {
		t.Fatalf("toast=%q", m.toast)
	}
	m.rows[0].result.SourceURL = "https://example.test/x"
	if cmd := m.openCurrent(); cmd == nil || !strings.Contains(m.toast, "opening") {
		t.Fatalf("toast=%q", m.toast)
	}
	m.cursor = 0
	m.items = nil
	if cmd := m.copyCurrent(); cmd != nil {
		t.Fatal("nil row should not copy")
	}
	if cmd := m.openCurrent(); cmd == nil {
		t.Fatal("nil row should still surface a toast")
	}
}

func TestDetailContentBranches(t *testing.T) {
	m := testModel()
	r := &row{
		job:    engine.Job{Category: check.CategoryDomains, Label: "acme.com", TargetLabel: "domain", Err: errors.New("bad")},
		status: check.StatusQueued,
		result: check.Result{
			Confidence: check.Certain, Summary: "registered", Cached: true,
			Latency: 20 * time.Millisecond, SourceURL: "https://example.test/acme.com",
			Fields: []check.Field{{Key: "registrar", Value: "Reg"}, {Key: "empty", Value: ""}, {Key: "note", Value: "a long value that wraps over multiple lines to exercise the wrapping path"}},
			Err:    errors.New("boom"),
		},
	}
	out := m.detailContent(r, 40)
	for _, want := range []string{"registered", "cached", "registrar", "boom", "example.test"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Fatal("escape survived")
	}

	r.status = check.StatusTaken
	r.result.Fields = []check.Field{{Key: "registrar", Value: "Reg"}}
	if out := m.detailContent(r, 40); !strings.Contains(out, "registration record") {
		t.Fatalf("registrar note missing: %q", out)
	}
	r.result.Fields = []check.Field{{Key: "dns", Value: "NS1"}}
	if out := m.detailContent(r, 40); !strings.Contains(out, "DNS delegation") {
		t.Fatalf("dns note missing: %q", out)
	}
	r.result.Fields = nil
	if out := m.detailContent(r, 40); !strings.Contains(out, "No metadata returned") {
		t.Fatalf("empty field note missing: %q", out)
	}
}

func TestPanelsAndOverlay(t *testing.T) {
	m := testModelWithRows()
	if got := m.detailPanel(); got != "" {
		t.Fatal("unexpected detail panel")
	}
	_ = m.openDetail(0)
	if got := m.detailPanel(); got == "" || !strings.Contains(got, "esc close") {
		t.Fatalf("panel=%q", got)
	}
	m.detail.row = 99
	if got := m.detailPanel(); got != "" {
		t.Fatal("out-of-range detail panel")
	}
	m.detail = nil

	m.helpOpen = true
	m.width, m.height = 80, 24
	if got := m.helpPanel(); !strings.Contains(got, "MOVE") || !strings.Contains(got, "ACTIONS") {
		t.Fatalf("help=%q", got)
	}
	m.width, m.height = 30, 8
	if got := m.helpPanel(); got == "" {
		t.Fatal("small help panel")
	}
	m.width, m.height = 80, 24
	m.detail = &detailState{row: 0}
	m.anim.detailPos = 100
	if got := m.overlay("base", "panel"); got == "" {
		t.Fatal("overlay empty")
	}
	if got := m.overlay("base", ""); got != "base" {
		t.Fatalf("overlay base=%q", got)
	}
	if got := m.overlay("one\ntwo", "very large panel content\nline two\nline three"); got == "" {
		t.Fatal("large overlay")
	}
	m.detail = nil
}

func TestDetailSizes(t *testing.T) {
	m := testModel()
	m.width, m.height = 40, 10
	if w, h := m.detailSize(); w <= 0 || h <= 0 {
		t.Fatalf("size=%dx%d", w, h)
	}
	m.width, m.height = 20, 4
	if w, h := m.detailSize(); w <= 0 || h <= 0 {
		t.Fatalf("size=%dx%d", w, h)
	}
	m.width, m.height = 10, 2
	if w, h := m.detailSize(); w <= 0 || h <= 0 {
		t.Fatalf("size=%dx%d", w, h)
	}
}

func TestExports(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	m := testModelWithRows()
	m.name = "Acme Inc"
	if cmd := m.exportJSON(); cmd == nil || !strings.Contains(m.toast, "wrote") {
		t.Fatalf("json toast=%q", m.toast)
	}
	if cmd := m.exportMarkdown(); cmd == nil || !strings.Contains(m.toast, "wrote") {
		t.Fatalf("md toast=%q", m.toast)
	}
	files, err := filepath.Glob(filepath.Join(dir, "nameary-*"))
	if err != nil || len(files) != 2 {
		t.Fatalf("files=%v err=%v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil || len(data) == 0 {
		t.Fatalf("read=%v err=%v", len(data), err)
	}
}

func TestWriteExportFailure(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)
	if _, err := writeExport("nameary-*.json", []byte("{}")); err == nil {
		t.Skip("directory permissions are not enforced in this environment")
	}
}

func TestSafeFileMore(t *testing.T) {
	if got := safeFile("  ACME_Inc!!  "); got != "acme-inc" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("a", 100)
	if got := safeFile(long); len(got) != 80 {
		t.Fatalf("len=%d", len(got))
	}
	if got := safeFile("---"); got != "results" {
		t.Fatalf("got %q", got)
	}
}

func TestRecords(t *testing.T) {
	m := testModelWithRows()
	m.rows[0].result.Err = errors.New("boom")
	m.rows[0].result.Fields = []check.Field{{Key: "k", Value: "v"}}
	recs := m.records()
	if len(recs) != 2 || recs[0].Error != "boom" || len(recs[0].Fields) != 1 {
		t.Fatalf("recs=%+v", recs)
	}
}

func TestThemeHelpers(t *testing.T) {
	th := newTheme(false, false)
	if th.style(nil).Render("x") != "x" || th.bold(nil).Render("x") == "" {
		t.Fatal("nil color should be unstyled")
	}
	for _, s := range []check.Status{
		check.StatusAvailable, check.StatusTaken, check.StatusUnknown, check.StatusError,
		check.StatusChecking, check.StatusSkipped, check.StatusCanceled, check.StatusQueued,
		check.StatusTimeout, check.StatusRateLimited,
	} {
		if th.statusColor(s) == nil || th.glyph(s, "") == "" || th.flashColor(s) == nil {
			t.Fatalf("missing theme data for %v", s)
		}
	}
	if th.glyph(check.StatusChecking, "X") != "X" {
		t.Fatal("spinner frame override")
	}
	th.setDark(false)
	if th.statusColor(check.StatusAvailable) == nil {
		t.Fatal("light theme color")
	}
	if th.borderStyle().Render("x") == "" || th.ellipsis() == "" {
		t.Fatal("border/ellipsis")
	}
	ascii := newTheme(true, true)
	if ascii.statusColor(check.StatusAvailable) != nil || ascii.flashColor(check.StatusTaken) != nil {
		t.Fatal("no-color theme returned colors")
	}
	if ascii.glyph(check.StatusAvailable, "") != "+" || ascii.glyph(check.StatusChecking, "") != "o" {
		t.Fatal("ascii glyphs")
	}
	if ascii.ellipsis() != "..." {
		t.Fatal("ascii ellipsis")
	}
}

func TestAnimTiming(t *testing.T) {
	now := time.Now()
	m := testModelWithRows()
	m.now = now

	m.toastUntil = now.Add(time.Second)
	if d, ok := m.tickDelay(); !ok || d != 200*time.Millisecond {
		t.Fatalf("toast delay=%v ok=%v", d, ok)
	}
	m.toastUntil = time.Time{}

	m.opts.ReducedMotion = true
	if _, ok := m.tickDelay(); ok {
		t.Fatal("reduced motion should not tick")
	}
	m.opts.ReducedMotion = false

	m.anim.logoStart = now.Add(-100 * time.Millisecond)
	if _, ok := m.tickDelay(); !ok {
		t.Fatal("logo animation should tick")
	}
	m.anim.logoStart = now.Add(-2 * time.Second)

	m.detail = &detailState{row: 0}
	m.anim.detailPos, m.anim.detailTarget = 0, 10
	if _, ok := m.tickDelay(); !ok {
		t.Fatal("detail animation should tick")
	}
	m.detail = nil
	m.anim.detailPos, m.anim.detailTarget = 0, 0

	m.anim.summaryStart = now.Add(-10 * time.Millisecond)
	if _, ok := m.tickDelay(); !ok {
		t.Fatal("summary animation should tick")
	}
	m.anim.summaryStart = time.Time{}

	m.rows[0].settledAt = now.Add(-10 * time.Millisecond)
	if _, ok := m.tickDelay(); !ok {
		t.Fatal("flash animation should tick")
	}
	m.rows[0].settledAt = time.Time{}

	m.phase = phaseRunning
	m.rows[0].status = check.StatusChecking
	if _, ok := m.tickDelay(); !ok {
		t.Fatal("pending run should tick")
	}
	m.rows[0].status = check.StatusAvailable
	m.rows[1].status = check.StatusTaken
	m.phase = phaseDone
	if _, ok := m.tickDelay(); ok {
		t.Fatal("idle model should not tick")
	}
}

func TestOnFrameAndFlash(t *testing.T) {
	now := time.Now()
	m := testModelWithRows()
	m.now = now
	m.detail = &detailState{row: 0}
	m.anim.detailPos, m.anim.detailTarget = 0, 100
	m.anim.summaryStart = now
	m.rows[0].settledAt = now
	_ = m.onFrame(now.Add(10 * time.Millisecond))
	if m.anim.frame == 0 {
		t.Fatal("frame not advanced")
	}
	if m.flashAmount(m.rows[0]) <= 0 {
		t.Fatal("expected flash")
	}
	m.rows[0].settledAt = now.Add(-time.Second)
	if m.flashAmount(m.rows[0]) != 0 {
		t.Fatal("old settle should not flash")
	}
	m.rows[0].settledAt = now.Add(time.Second)
	if m.flashAmount(m.rows[0]) != 0 {
		t.Fatal("future settle should not flash")
	}
	m.opts.ReducedMotion = true
	if m.flashAmount(m.rows[0]) != 0 {
		t.Fatal("reduced motion should not flash")
	}
	m.opts.ReducedMotion = false

	m.toast = "hi"
	m.toastUntil = now.Add(time.Millisecond)
	_ = m.onFrame(now.Add(time.Second))
	if m.toast != "" {
		t.Fatal("toast should expire")
	}
	if m.hasPending() {
		t.Fatal("all rows are final")
	}
	m.rows[0].status = check.StatusChecking
	if !m.hasPending() {
		t.Fatal("checking row is pending")
	}
}

func TestViewHelpers(t *testing.T) {
	m := testModelWithRows()
	m.opts.NoColor = false
	m.opts.ReducedMotion = true
	m.anim.logoStart = time.Now()
	if got := m.logo(); got == "" {
		t.Fatal("empty logo")
	}
	m.opts.NoColor = true
	if got := m.logo(); !strings.Contains(got, "nameary") {
		t.Fatalf("logo=%q", got)
	}

	m.width = 80
	if got := m.progressLine(80); got == "" {
		t.Fatal("progress line")
	}
	m.opts.ReducedMotion = false
	m.phase = phaseRunning
	m.rows[0].status = check.StatusChecking
	if got := m.progressLine(80); got == "" {
		t.Fatal("animated progress line")
	}
	m.opts.ASCII = true
	m.opts.NoColor = true
	if got := m.progressLine(80); !strings.Contains(got, "[") {
		t.Fatalf("ascii progress=%q", got)
	}

	m.counts = map[check.Status]int{check.StatusAvailable: 1, check.StatusCanceled: 2}
	m.settled = 1
	if got := m.statsLine(80); !strings.Contains(got, "skipped") || !strings.Contains(got, "in flight") {
		t.Fatalf("stats=%q", got)
	}

	if got := m.groupLine(item{group: check.CategoryDomains}, 40); got == "" {
		t.Fatal("group line")
	}
	m.opts.ASCII = false
	m.opts.NoColor = false
	m.rows[0].status = check.StatusChecking
	m.rows[0].job.Err = errors.New("bad")
	m.rows[0].result.SourceURL = "https://example.test/x"
	m.cursor = firstRow(m.items)
	if got := m.rowLine(m.rows[0], 90, true); got == "" || !strings.Contains(got, "↗") {
		t.Fatalf("row=%q", got)
	}
	if got := m.rowLine(m.rows[0], 50, false); got == "" {
		t.Fatalf("compact row=%q", got)
	}
	m.rows[0].settledAt = time.Now()
	m.now = time.Now()
	if got := m.rowLine(m.rows[0], 90, true); got == "" {
		t.Fatal("flash row")
	}

	for _, s := range []check.Status{
		check.StatusQueued, check.StatusChecking, check.StatusAvailable, check.StatusTaken,
		check.StatusUnknown, check.StatusError, check.StatusSkipped, check.StatusTimeout, check.StatusRateLimited,
	} {
		r := &row{status: s, job: engine.Job{Category: check.CategoryDomains}, result: check.Result{Err: errors.New("boom")}}
		if got := m.statusMeta(r); got == "" {
			t.Fatalf("status meta for %v", s)
		}
	}

	m.filterOn = true
	m.toast = "toast"
	if got := strings.Join(m.footerLines(80), "\n"); !strings.Contains(got, "toast") {
		t.Fatalf("footer=%q", got)
	}
	m.filterOn = false
	m.toast = ""
	m.phase = phaseDone
	if got := strings.Join(m.footerLines(100), "\n"); !strings.Contains(got, "export") {
		t.Fatalf("footer=%q", got)
	}
	if got := m.footerLines(50); len(got) == 0 {
		t.Fatal("narrow footer")
	}
	m.opts.ASCII = true
	if got := strings.Join(m.footerLines(100), "\n"); !strings.Contains(got, "enter details") {
		t.Fatalf("ascii footer=%q", got)
	}
}

func TestListAndBody(t *testing.T) {
	m := testModelWithRows()
	m.height = 12
	m.rebuildItems()
	if got := m.listLines(80); len(got) == 0 {
		t.Fatal("empty list")
	}
	m.items = nil
	if got := m.listLines(80); !strings.Contains(strings.Join(got, ""), "no results") {
		t.Fatalf("empty list lines=%v", got)
	}

	m = testModelWithRows()
	m.rebuildItems()
	m.cursor = firstRow(m.items)
	if got := m.sideLines(40); len(got) == 0 {
		t.Fatal("empty side lines")
	}
	m.cursor = 0
	if got := m.sideLines(40); len(got) == 0 {
		t.Fatal("no-cursor side lines")
	}

	m.width, m.height = 100, 24
	if got := m.bodyLines(98, 10); len(got) != 10 {
		t.Fatalf("body lines=%d", len(got))
	}
	m.width, m.height = 80, 24
	if got := m.bodyLines(70, 10); len(got) != 10 {
		t.Fatalf("narrow body lines=%d", len(got))
	}

	if got := m.frame("title", []string{"a", "b"}, 20, 6); got == "" {
		t.Fatal("frame")
	}
	m.opts.ASCII = true
	if got := m.frame("", []string{"a"}, 20, 6); got == "" {
		t.Fatal("ascii frame")
	}
	if got := m.frame("this is a very long title", nil, 4, 1); got == "" {
		t.Fatal("tiny frame")
	}
}

func TestInputBodyMore(t *testing.T) {
	m := newModel(context.Background(), Options{NoColor: true, ASCII: true, Targets: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}})
	m.width, m.height = 80, 24
	m.inputErr = errors.New("bad input")
	body := m.inputBody(80, 24)
	if len(body) == 0 || len(body) > 24 {
		t.Fatalf("lines=%d", len(body))
	}
	if !strings.Contains(strings.Join(body, "\n"), "targets selected") {
		t.Fatalf("body=%v", body)
	}
	if got := m.inputBody(80, 5); len(got) != 5 {
		t.Fatalf("short body=%d", len(got))
	}
}

func TestAppendHelpers(t *testing.T) {
	m := testModel()
	lines := appendKV(nil, m, "key", "a long value for wrapping around", 20)
	if len(lines) < 2 {
		t.Fatalf("lines=%v", lines)
	}
	lines = appendWrapped(nil, m.theme.style(m.theme.text), "a long value for wrapping", 10)
	if len(lines) < 2 {
		t.Fatalf("lines=%v", lines)
	}
	if got := wrapText("", 10); got != "" {
		t.Fatalf("wrap=%q", got)
	}
	if got := wrapText("abc", 0); got != "abc" {
		t.Fatalf("wrap=%q", got)
	}
	if got := truncateText("abcdef", 3); got != "ab…" {
		t.Fatalf("truncate=%q", got)
	}
	if got := truncateText("a", 0); got != "" {
		t.Fatalf("truncate=%q", got)
	}
	if got := center("x", 5); got != "  x" {
		t.Fatalf("center=%q", got)
	}
	if got := center("long", 2); got != "long" {
		t.Fatalf("center=%q", got)
	}
	if got := fit("a", 0); got != "" {
		t.Fatalf("fit=%q", got)
	}
	if got := padPlain("a", 0); got != "" {
		t.Fatalf("pad=%q", got)
	}
	if got := joinLR("a", "b", 0); got != "" {
		t.Fatalf("join=%q", got)
	}
	if got := joinLR("a", "b", 10); !strings.Contains(got, "a") {
		t.Fatalf("join=%q", got)
	}
	if min(1, 2) != 1 || max(1, 2) != 2 {
		t.Fatal("min/max")
	}
}

func TestViewOverlays(t *testing.T) {
	m := testModelWithRows()
	m.helpOpen = true
	content := m.View().Content
	if !strings.Contains(content, "help") {
		t.Fatalf("help overlay missing: %s", content)
	}
	m.helpOpen = false
	_ = m.openDetail(0)
	content = m.View().Content
	if !strings.Contains(content, "esc close") {
		t.Fatalf("detail overlay missing: %s", content)
	}
	m.detail = nil
	m.phase = phaseRunning
	m.rows[0].status = check.StatusChecking
	content = m.View().Content
	if !strings.Contains(content, "acme") {
		t.Fatalf("running view missing: %s", content)
	}
}

func TestScreenAndBodySmall(t *testing.T) {
	m := testModel()
	if got := m.screen([]string{"a"}, 10, 3); strings.Count(got, "\n") != 2 {
		t.Fatalf("screen=%q", got)
	}
	m.width, m.height = 20, 6
	_ = m.View()
	if got := m.mainScreen(20, 6); got == "" {
		t.Fatal("small main screen")
	}
}

func TestFrameExtraBranches(t *testing.T) {
	m := testModel()
	m.opts.ASCII = false
	m.theme.noColor = true
	got := m.frame("title", nil, 10, 4)
	if got == "" {
		t.Fatal("empty frame")
	}
	if lines := strings.Split(got, "\n"); len(lines) != 4 || lipgloss.Width(lines[0]) < 10 {
		t.Fatalf("frame=%q", got)
	}
}

func TestTickIfNeeded(t *testing.T) {
	m := testModel()
	m.now = time.Now()
	m.anim.logoStart = m.now.Add(-100 * time.Millisecond)
	if cmd := m.tickIfNeeded(); cmd == nil {
		t.Fatal("expected animation tick")
	}
	m.opts.ReducedMotion = true
	if cmd := m.tickIfNeeded(); cmd != nil {
		t.Fatal("reduced motion should not tick")
	}
}
