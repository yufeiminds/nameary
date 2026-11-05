package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/engine"
)

func fixture() ([]engine.Job, <-chan engine.Event) {
	jobs := []engine.Job{
		{ID: "1", Target: check.TargetNPM, TargetLabel: "npm", Label: "acme", Raw: "acme", Query: "acme"},
		{ID: "2", Target: check.TargetDomains, TargetLabel: "domain", Label: "acme.com", Raw: "acme", Query: "acme.com"},
	}
	ch := make(chan engine.Event, 2)
	ch <- engine.Event{Kind: engine.EventSettled, Job: jobs[0], Result: check.Result{
		Status: check.StatusAvailable, Confidence: check.Certain, Query: "acme",
		Latency: 120 * time.Millisecond, SourceURL: "https://npmjs.com/acme",
	}}
	ch <- engine.Event{Kind: engine.EventSettled, Job: jobs[1], Result: check.Result{
		Status: check.StatusTaken, Confidence: check.Certain, Query: "acme.com",
		Latency: 300 * time.Millisecond, SourceURL: "https://rdap.example/acme.com",
	}}
	close(ch)
	return jobs, ch
}

func TestTable(t *testing.T) {
	jobs, ch := fixture()
	var out, errOut bytes.Buffer
	res, err := Run(ch, jobs, Options{Format: "table", NoColor: true, Out: &out, Err: &errOut})
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts[check.StatusAvailable] != 1 || res.Counts[check.StatusTaken] != 1 {
		t.Fatalf("counts=%v", res.Counts)
	}
	text := out.String()
	if !strings.Contains(text, "npm") || !strings.Contains(text, "acme.com") {
		t.Fatalf("missing rows: %s", text)
	}
	if !strings.Contains(errOut.String(), "1 available") {
		t.Fatalf("missing summary: %s", errOut.String())
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatal("table should be plain with NoColor")
	}
}

func TestNDJSONAndFilter(t *testing.T) {
	jobs, ch := fixture()
	var out bytes.Buffer
	res, err := Run(ch, jobs, Options{Format: "ndjson", OnlyAvailable: true, Out: &out})
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts[check.StatusTaken] != 1 {
		t.Fatalf("counts should include filtered rows: %v", res.Counts)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("lines=%d: %s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `"status":"available"`) {
		t.Fatalf("line=%s", lines[0])
	}
}

func TestJSON(t *testing.T) {
	jobs, ch := fixture()
	var out bytes.Buffer
	if _, err := Run(ch, jobs, Options{Format: "json", Out: &out}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.HasPrefix(strings.TrimSpace(text), "[") || !strings.Contains(text, `"acme.com"`) {
		t.Fatalf("bad json: %s", text)
	}
}

func TestTableSanitizesUntrustedText(t *testing.T) {
	jobs := []engine.Job{{ID: "1", Target: check.TargetNPM, TargetLabel: "npm\x1b[2J", Label: "react\x1b[31m", Raw: "react", Query: "react"}}
	ch := make(chan engine.Event, 1)
	ch <- engine.Event{Kind: engine.EventSettled, Job: jobs[0], Result: check.Result{
		Status: check.StatusTaken, Summary: "x\x1b[2J", SourceURL: "https://example.test/\x1b[2J", Latency: time.Millisecond,
	}}
	close(ch)
	var out bytes.Buffer
	if _, err := Run(ch, jobs, Options{Format: "table", NoColor: true, Out: &out}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatalf("terminal escape survived: %q", out.String())
	}
}
