package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/engine"
)

func testModel() *model {
	m := newModel(context.Background(), Options{NoColor: true, ASCII: true})
	m.width, m.height = 80, 24
	m.phase = phaseDone
	m.name = "acme"
	m.started = time.Now().Add(-time.Second)
	m.finished = time.Now()
	m.rows = []*row{
		{job: engine.Job{ID: "1", Target: check.TargetDomains, TargetLabel: "domain", Category: check.CategoryDomains, Label: "acme.com", Raw: "acme", Query: "acme.com"}, status: check.StatusTaken, result: check.Result{Status: check.StatusTaken, Confidence: check.Certain, Summary: "registered", Latency: 120 * time.Millisecond}},
		{job: engine.Job{ID: "2", Target: check.TargetNPM, TargetLabel: "npm", Category: check.CategoryPackages, Label: "acme", Raw: "acme", Query: "acme"}, status: check.StatusAvailable, result: check.Result{Status: check.StatusAvailable, Confidence: check.Certain, Summary: "no record", Latency: 80 * time.Millisecond}},
	}
	m.counts = map[check.Status]int{check.StatusTaken: 1, check.StatusAvailable: 1}
	m.settled = 2
	m.items = nil
	m.rebuildItems()
	m.prog.SetWidth(60)
	return m
}

func TestViewFitsTerminal(t *testing.T) {
	m := testModel()
	content := m.View().Content
	lines := strings.Split(content, "\n")
	if len(lines) != 24 {
		t.Fatalf("lines=%d, want 24", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 80 {
			t.Fatalf("line %d width=%d, want 80: %q", i, w, line)
		}
	}
	if !strings.Contains(content, "DOMAINS") || !strings.Contains(content, "PACKAGES") {
		t.Fatalf("missing groups: %s", content)
	}
}

func TestNarrowView(t *testing.T) {
	m := testModel()
	m.width, m.height = 52, 18
	content := m.View().Content
	for i, line := range strings.Split(content, "\n") {
		if w := lipgloss.Width(line); w != 52 {
			t.Fatalf("line %d width=%d, want 52", i, w)
		}
	}
}

func TestFilter(t *testing.T) {
	m := testModel()
	m.filter.SetValue("npm")
	m.rebuildItems()
	var rows int
	for _, it := range m.items {
		if it.kind == itemRow {
			rows++
			if m.rows[it.row].job.TargetLabel != "npm" {
				t.Fatalf("unexpected row %q", m.rows[it.row].job.TargetLabel)
			}
		}
	}
	if rows != 1 {
		t.Fatalf("rows=%d, want 1", rows)
	}
}

func TestOnlyAvailable(t *testing.T) {
	m := testModel()
	m.onlyAvailable = true
	m.rebuildItems()
	for _, it := range m.items {
		if it.kind == itemRow && m.rows[it.row].status != check.StatusAvailable {
			t.Fatalf("non-available row survived filter")
		}
	}
}

func TestEmptyNameShowsError(t *testing.T) {
	m := newModel(context.Background(), Options{NoColor: true, ASCII: true})
	m.phase = phaseInput
	m.input.SetValue("")
	_, _ = m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.inputErr == nil {
		t.Fatal("expected an input error")
	}
}

func TestDetailOverlay(t *testing.T) {
	m := testModel()
	_ = m.openDetail(0)
	content := m.View().Content
	if !strings.Contains(content, "esc close") {
		t.Fatalf("detail footer missing: %s", content)
	}
	for i, line := range strings.Split(content, "\n") {
		if w := lipgloss.Width(line); w != 80 {
			t.Logf("line %d = %q", i, line)
			t.Fatalf("line %d width=%d", i, w)
		}
	}
}

func TestSafeFile(t *testing.T) {
	if got := safeFile("Acme / Inc"); got != "acme---inc" {
		t.Fatalf("got %q", got)
	}
	if got := safeFile(""); got != "results" {
		t.Fatalf("got %q", got)
	}
}

func TestInputView(t *testing.T) {
	m := newModel(context.Background(), Options{NoColor: true, ASCII: true, Targets: []string{"domains", "npm"}, TLDs: []string{"com", "io"}})
	m.width, m.height = 80, 24
	content := m.View().Content
	lines := strings.Split(content, "\n")
	if len(lines) != 24 {
		t.Fatalf("lines=%d", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 80 {
			t.Fatalf("line %d width=%d", i, w)
		}
	}
	plain := ansi.Strip(content)
	if !strings.Contains(plain, "nameary") || !strings.Contains(plain, "> acme") {
		t.Fatalf("missing input UI: %s", plain)
	}
}

func TestWideViewHasSidebar(t *testing.T) {
	m := testModel()
	m.width, m.height = 120, 30
	content := m.View().Content
	if !strings.Contains(content, "SELECTED") || !strings.Contains(content, "AVAILABLE") {
		t.Fatalf("missing sidebar: %s", content)
	}
	lines := strings.Split(content, "\n")
	if len(lines) != 30 {
		t.Fatalf("lines=%d", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 120 {
			t.Fatalf("line %d width=%d", i, w)
		}
	}
}

func TestViewSanitizesUntrustedText(t *testing.T) {
	m := testModel()
	m.name = "acme\x1b[2J"
	m.rows[0].job.Label = "acme.com\x1b[31m"
	m.rows[0].job.TargetLabel = "domain\x1b[2J"
	m.rows[0].result.Summary = "summary\x1b[2J"
	m.rows[0].result.Fields = []check.Field{{Key: "k\x1b[2J", Value: "v\x1b[2J"}}
	content := m.View().Content
	if strings.Contains(content, "\x1b") {
		t.Fatalf("terminal escape survived: %q", content)
	}
}

func TestSafeExternalURL(t *testing.T) {
	for _, ok := range []string{"https://example.test/x", "http://example.test/"} {
		if _, valid := safeExternalURL(ok); !valid {
			t.Fatalf("expected valid: %s", ok)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "file:///etc/passwd", "ftp://example.test/x", "https://", "/relative"} {
		if _, valid := safeExternalURL(bad); valid {
			t.Fatalf("expected invalid: %s", bad)
		}
	}
}
