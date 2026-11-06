package render

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/engine"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunErrors(t *testing.T) {
	jobs, ch := fixture()
	if _, err := Run(ch, jobs, Options{Format: "bogus"}); err == nil {
		t.Fatal("expected format error")
	}
	jobs, ch = fixture()
	if _, err := Run(ch, jobs, Options{Format: "json", Out: failingWriter{}}); err == nil {
		t.Fatal("expected json write error")
	}
	jobs, ch = fixture()
	if _, err := Run(ch, jobs, Options{Format: "ndjson", Out: failingWriter{}}); err == nil {
		t.Fatal("expected ndjson write error")
	}
}

func TestRunSummaryBranches(t *testing.T) {
	jobs := []engine.Job{{ID: "1", Target: check.TargetNPM, TargetLabel: "npm", Label: "acme", Raw: "acme"}}
	ch := make(chan engine.Event, 2)
	ch <- engine.Event{Kind: engine.EventSettled, Job: jobs[0], Result: check.Result{Status: check.StatusError, Err: errors.New("boom")}}
	ch <- engine.Event{Kind: engine.EventSettled, Job: jobs[0], Result: check.Result{Status: check.StatusSkipped}}
	close(ch)
	var out, errOut bytes.Buffer
	if _, err := Run(ch, jobs, Options{Format: "table", NoColor: true, Out: &out, Err: &errOut}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "error") || !strings.Contains(errOut.String(), "skipped") {
		t.Fatalf("summary=%q", errOut.String())
	}
}

func TestRunDefaults(t *testing.T) {
	jobs, ch := fixture()
	if _, err := Run(ch, jobs, Options{}); err != nil {
		t.Fatal(err)
	}
}

func TestGlyphFor(t *testing.T) {
	for _, ascii := range []bool{true, false} {
		for _, s := range []check.Status{
			check.StatusAvailable, check.StatusTaken, check.StatusUnknown, check.StatusError,
			check.StatusSkipped, check.StatusCanceled, check.StatusQueued, check.StatusTimeout, check.StatusRateLimited, check.StatusChecking,
			check.StatusTimeout, check.StatusRateLimited,
		} {
			g, c := glyphFor(s, ascii)
			if g == "" || c == "" {
				t.Fatalf("ascii=%v status=%v glyph=%q color=%q", ascii, s, g, c)
			}
		}
	}
}

func TestTableWriterWidths(t *testing.T) {
	jobs := []engine.Job{{TargetLabel: strings.Repeat("t", 80), Label: strings.Repeat("n", 80)}}
	tw := newTableWriter(&bytes.Buffer{}, jobs, true, true)
	if tw.nameW > 42 || tw.targetW != 80 {
		t.Fatalf("nameW=%d targetW=%d", tw.nameW, tw.targetW)
	}
}

func TestConsolePaletteAndGlyphs(t *testing.T) {
	p := newConsolePalette(false, true)
	for _, s := range []check.Status{
		check.StatusAvailable, check.StatusTaken, check.StatusUnknown, check.StatusError,
		check.StatusSkipped, check.StatusCanceled, check.StatusQueued, check.StatusTimeout, check.StatusRateLimited,
	} {
		if p.statusColor(s) == nil {
			t.Fatalf("missing color for %v", s)
		}
		if p.style(p.statusColor(s)).Render("x") == "" {
			t.Fatal("empty render")
		}
	}
	nc := newConsolePalette(true, false)
	for _, s := range []check.Status{check.StatusAvailable, check.StatusQueued} {
		if nc.statusColor(s) != nil {
			t.Fatalf("no-color palette returned a color for %v", s)
		}
	}
	if nc.style(nil).Render("x") != "x" || nc.bold(nil).Render("x") != "x" {
		t.Fatal("no-color styling should be plain")
	}
	for _, ascii := range []bool{true, false} {
		for _, s := range []check.Status{
			check.StatusAvailable, check.StatusTaken, check.StatusUnknown, check.StatusError,
			check.StatusSkipped, check.StatusCanceled, check.StatusQueued, check.StatusTimeout, check.StatusRateLimited,
		} {
			if consoleGlyph(s, ascii) == "" {
				t.Fatalf("empty glyph for %v", s)
			}
		}
	}
}

func TestConsoleStatusHelpers(t *testing.T) {
	p := newConsolePalette(true, false)
	var buf bytes.Buffer
	st := &consoleStatus{out: &buf, live: true, p: p, width: 80, total: 3}
	counts := map[check.Status]int{check.StatusAvailable: 1}
	st.draw(counts, 1, "npm taken", time.Now(), false)
	st.clear()
	if buf.Len() == 0 || !strings.Contains(buf.String(), "1/3") {
		t.Fatalf("status output=%q", buf.String())
	}
	st.live = false
	buf.Reset()
	st.draw(counts, 1, "x", time.Now(), false)
	st.clear()
	if buf.Len() != 0 {
		t.Fatalf("non-live status wrote %q", buf.String())
	}

	line := consoleStatusLine(map[check.Status]int{
		check.StatusAvailable: 1, check.StatusTaken: 1, check.StatusUnknown: 1,
		check.StatusError: 1, check.StatusSkipped: 1, check.StatusCanceled: 1,
		check.StatusTimeout: 1, check.StatusRateLimited: 1,
	}, 8, 8, "", 0, 140, p, true, time.Now().Add(-time.Second), true)
	if !strings.Contains(line, "available") || !strings.Contains(line, "timeout") ||
		!strings.Contains(line, "rate limit") || !strings.Contains(line, "skipped") {
		t.Fatalf("line=%q", line)
	}
	line = consoleStatusLine(counts, 1, 3, "npm taken", 2, 40, p, true, time.Now(), false)
	if !strings.Contains(line, "/") || !strings.Contains(line, "npm taken") {
		t.Fatalf("line=%q", line)
	}
}

func TestConsoleSummaryCelebrate(t *testing.T) {
	p := newConsolePalette(true, false)
	out := consoleSummary(Result{Counts: map[check.Status]int{check.StatusAvailable: 2}, Total: 2}, p, true, true, 80)
	if !strings.Contains(out, "all clear") {
		t.Fatalf("missing celebration: %q", out)
	}
	out = consoleSummary(Result{Counts: map[check.Status]int{
		check.StatusAvailable: 1, check.StatusTaken: 1, check.StatusUnknown: 1,
		check.StatusError: 1, check.StatusCanceled: 1,
	}}, p, false, true, 80)
	if strings.Contains(out, "all clear") || !strings.Contains(out, "error") || !strings.Contains(out, "skipped") {
		t.Fatalf("summary=%q", out)
	}
}

func TestConsoleCellsAndRows(t *testing.T) {
	p := newConsolePalette(true, false)
	job := engine.Job{ID: "1", TargetLabel: "npm", Category: check.CategoryPackages, Label: "acme", Query: "acme"}
	waiting := consoleCell(job, check.Result{}, false, 32, p, true)
	if !strings.Contains(waiting, "waiting") {
		t.Fatalf("waiting=%q", waiting)
	}
	r := check.Result{Status: check.StatusAvailable, Confidence: check.Likely, Summary: "no record", Cached: true, Latency: time.Millisecond}
	cell := consoleCell(job, r, true, 90, p, false)
	if !strings.Contains(cell, "availabl") || !strings.Contains(cell, "cached") {
		t.Fatalf("cell=%q", cell)
	}
	short := consoleCell(job, r, true, 20, p, false)
	if short == "" {
		t.Fatal("short cell")
	}
	row := consoleRow(job, check.Result{Status: check.StatusTaken, Summary: strings.Repeat("long summary ", 20)}, 60, p, false)
	if row == "" {
		t.Fatal("empty row")
	}
	domainJob := engine.Job{ID: "2", TargetLabel: "domain", Category: check.CategoryDomains, Label: "acme.com"}
	if got := consoleLabel(domainJob); got != "acme.com" {
		t.Fatalf("label=%q", got)
	}
	if got := consoleLabel(job); got != "npm  acme" {
		t.Fatalf("label=%q", got)
	}
}

func TestConsoleMeta(t *testing.T) {
	if got := consoleMeta(check.Result{}, false); got != "" {
		t.Fatalf("meta=%q", got)
	}
	got := consoleMeta(check.Result{
		Status: check.StatusAvailable, Confidence: check.Likely, Summary: "free",
		Latency: 20 * time.Millisecond,
	}, true)
	for _, want := range []string{"free", "likely", "20ms", "|"} {
		if !strings.Contains(got, want) {
			t.Fatalf("meta=%q missing %q", got, want)
		}
	}
	if got := consoleMeta(check.Result{Status: check.StatusTaken, Cached: true}, false); !strings.Contains(got, "cached") {
		t.Fatalf("meta=%q", got)
	}
}

func TestConsoleGroupsAndCategories(t *testing.T) {
	jobs, _ := consoleFixture()
	jobs = append(jobs, engine.Job{ID: "4", Category: check.Category("OTHER"), TargetLabel: "other", Label: "x"})
	cats := consoleCategories(jobs)
	if len(cats) != 4 {
		t.Fatalf("cats=%v", cats)
	}
	for i := 1; i < len(cats); i++ {
		if catRank(cats[i-1]) > catRank(cats[i]) {
			t.Fatalf("unsorted: %v", cats)
		}
	}
	if got := consoleGroups(map[check.Category]int{
		check.CategoryDomains: 1, check.CategoryPackages: 2, check.CategoryHandles: 3,
	}, newConsolePalette(true, false), true); !strings.Contains(got, "|") {
		t.Fatalf("groups=%q", got)
	}
	if got := consoleGroups(nil, newConsolePalette(true, false), false); got != "" {
		t.Fatalf("groups=%q", got)
	}
	if countCategory(jobs, check.CategoryPackages) != 1 {
		t.Fatal("countCategory")
	}
	if got := jobsByCategory(jobs, check.CategoryDomains); len(got) != 1 {
		t.Fatalf("jobsByCategory=%v", got)
	}
}

func TestConsoleTextHelpers(t *testing.T) {
	if ruleChar(true) != "-" || ruleChar(false) != "─" {
		t.Fatal("ruleChar")
	}
	if got := truncatePlain("abcdef", 3); got != "ab…" {
		t.Fatalf("truncate=%q", got)
	}
	if got := truncatePlain("abc", 0); got != "" {
		t.Fatalf("truncate=%q", got)
	}
	if got := padPlain("abcdef", 3); got != "ab…" {
		t.Fatalf("pad=%q", got)
	}
	if got := padPlain("a", 3); got != "a  " {
		t.Fatalf("pad=%q", got)
	}
	if got := padPlain("a", 0); got != "" {
		t.Fatalf("pad=%q", got)
	}
	if got := joinLR("left", "right", 20); got != "left"+strings.Repeat(" ", 11)+"right" {
		t.Fatalf("join=%q", got)
	}
	if got := joinLR("left", "right", 4); got != "left" {
		t.Fatalf("join narrow=%q", got)
	}
	if got := joinLR("a", "b", 0); got != "" {
		t.Fatalf("join zero=%q", got)
	}
	if got := fit("abcdef", 3); got != "ab…" {
		t.Fatalf("fit=%q", got)
	}
	if got := fit("a", 0); got != "" {
		t.Fatalf("fit zero=%q", got)
	}
}

func TestConsoleLogoModes(t *testing.T) {
	for _, tc := range []struct {
		noColor bool
		ascii   bool
		compact bool
	}{{true, true, true}, {true, false, false}, {false, true, false}, {false, false, true}} {
		got := consoleLogo(newConsolePalette(tc.noColor, false), tc.ascii, tc.compact)
		if got == "" {
			t.Fatalf("logo empty for %+v", tc)
		}
	}
}

func TestConsoleLiveFallback(t *testing.T) {
	jobs, ch := consoleFixture()
	var out bytes.Buffer
	// A short terminal forces the grouped fallback even with Live set.
	if _, err := RunConsole(ch, jobs, ConsoleOptions{
		NoColor: true, ASCII: true, Width: 60, Height: 8, Live: true, Out: &out,
	}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "\x1b[?25l") || !strings.Contains(text, "DOMAINS") {
		t.Fatalf("fallback output=%q", text)
	}
}

func TestConsoleLayoutNoFit(t *testing.T) {
	jobs, _ := consoleFixture()
	if _, _, ok := consoleLayout(jobs, 40, 4, newConsolePalette(true, false), true); ok {
		t.Fatal("expected layout not to fit")
	}
}

func TestConsoleGroupLinesOnlyAvailable(t *testing.T) {
	jobs, _ := consoleFixture()
	results := map[string]check.Result{jobs[0].ID: {Status: check.StatusAvailable}}
	lines := consoleGroupLines(jobs, results, check.CategoryPackages, 80, newConsolePalette(true, false), true, true)
	if len(lines) != 0 {
		t.Fatalf("expected no lines, got %v", lines)
	}
	lines = consoleGroupLines(jobs, results, check.CategoryDomains, 80, newConsolePalette(true, false), true, false)
	if len(lines) != 2 {
		t.Fatalf("lines=%v", lines)
	}
}
