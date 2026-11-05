package render

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/engine"
)

func consoleFixture() ([]engine.Job, <-chan engine.Event) {
	jobs := []engine.Job{
		{ID: "1", Target: check.TargetDomains, TargetLabel: "domain", Category: check.CategoryDomains, Label: "acme.com", Raw: "acme", Query: "acme.com"},
		{ID: "2", Target: check.TargetNPM, TargetLabel: "npm", Category: check.CategoryPackages, Label: "acme", Raw: "acme", Query: "acme"},
		{ID: "3", Target: check.TargetGitHub, TargetLabel: "github", Category: check.CategoryHandles, Label: "acme", Raw: "acme", Query: "acme"},
	}
	ch := make(chan engine.Event, 3)
	ch <- engine.Event{Kind: engine.EventSettled, Job: jobs[0], Result: check.Result{
		Status: check.StatusAvailable, Confidence: check.Likely, Summary: "no registration record", Latency: 120 * time.Millisecond,
	}}
	ch <- engine.Event{Kind: engine.EventSettled, Job: jobs[1], Result: check.Result{
		Status: check.StatusTaken, Confidence: check.Certain, Summary: "v3.0.3", Latency: 80 * time.Millisecond,
	}}
	ch <- engine.Event{Kind: engine.EventSettled, Job: jobs[2], Result: check.Result{
		Status: check.StatusAvailable, Confidence: check.Certain, Summary: "no record", Latency: 200 * time.Millisecond, Cached: true,
	}}
	close(ch)
	return jobs, ch
}

func TestConsole(t *testing.T) {
	jobs, ch := consoleFixture()
	var out bytes.Buffer
	res, err := RunConsole(ch, jobs, ConsoleOptions{
		NoColor: true, ASCII: true, Width: 100, Out: &out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts[check.StatusAvailable] != 2 || res.Counts[check.StatusTaken] != 1 {
		t.Fatalf("counts=%v", res.Counts)
	}
	text := out.String()
	for _, want := range []string{"nameary", "acme", "DOMAINS", "PACKAGES", "HANDLES", "acme.com", "npm", "github", "2", "1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("expected plain output:\n%s", text)
	}
	if strings.Index(text, "DOMAINS") > strings.Index(text, "acme.com") {
		t.Fatalf("header/groups should print before results:\n%s", text)
	}
}

func TestConsoleOnlyAvailable(t *testing.T) {
	jobs, ch := consoleFixture()
	var out bytes.Buffer
	if _, err := RunConsole(ch, jobs, ConsoleOptions{
		NoColor: true, ASCII: true, Width: 100, OnlyAvailable: true, Out: &out,
	}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "v3.0.3") || strings.Contains(text, "npm  acme") {
		t.Fatalf("taken row survived:\n%s", text)
	}
}

func TestConsoleLive(t *testing.T) {
	jobs, ch := consoleFixture()
	var out bytes.Buffer
	if _, err := RunConsole(ch, jobs, ConsoleOptions{
		NoColor: true, ASCII: true, Width: 100, Live: true, Out: &out,
	}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "\x1b[?25l") || !strings.Contains(text, "\x1b[?25h") {
		t.Fatalf("missing cursor handling: %q", text)
	}
	if !strings.Contains(text, "\r") {
		t.Fatalf("missing status line: %q", text)
	}
}

func TestConsoleBlockHeight(t *testing.T) {
	for _, n := range []int{3, 25} {
		jobs := make([]engine.Job, n)
		for i := range jobs {
			cat := check.CategoryPackages
			if i%5 == 0 {
				cat = check.CategoryDomains
			}
			jobs[i] = engine.Job{ID: fmt.Sprint(i), Category: cat, TargetLabel: "npm", Label: "name", Query: "name"}
		}
		cols, h, ok := consoleLayout(jobs, 80, 24, newConsolePalette(n == 3, false), false)
		if !ok {
			t.Fatalf("n=%d layout should fit", n)
		}
		lines := consoleBlockLines(jobs, nil, map[check.Status]int{}, 0, "", 0, 80, cols, newConsolePalette(n == 3, false), false, time.Now(), false)
		if len(lines) != h {
			t.Fatalf("n=%d lines=%d layoutH=%d cols=%d", n, len(lines), h, cols)
		}
	}
}

func TestConsoleHeaderHeight(t *testing.T) {
	jobs, _ := consoleFixture()
	for _, nc := range []bool{true, false} {
		p := newConsolePalette(nc, false)
		h := len(consoleHeaderLines(jobs, 80, p, false, false, true))
		t.Logf("noColor=%v headerLines=%d", nc, h)
	}
}

func TestConsoleSanitizesUntrustedText(t *testing.T) {
	jobs := []engine.Job{{ID: "1", Category: check.CategoryDomains, TargetLabel: "domain", Label: "acme.com\x1b[2J", Raw: "acme\x1b[2J", Query: "acme.com"}}
	ch := make(chan engine.Event, 1)
	ch <- engine.Event{Kind: engine.EventSettled, Job: jobs[0], Result: check.Result{
		Status: check.StatusAvailable, Confidence: check.Certain, Summary: "summary\x1b[2J", Latency: time.Millisecond,
	}}
	close(ch)
	var out bytes.Buffer
	if _, err := RunConsole(ch, jobs, ConsoleOptions{NoColor: true, ASCII: true, Width: 100, Height: 40, Out: &out}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatalf("terminal escape survived: %q", out.String())
	}
}
