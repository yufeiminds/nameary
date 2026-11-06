// Package render provides streaming table, JSON and NDJSON output.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/engine"
	"github.com/yufeiminds/nameary/internal/textutil"
)

// Options controls headless output.
type Options struct {
	Format        string
	OnlyAvailable bool
	NoColor       bool
	ASCII         bool
	Out           io.Writer
	Err           io.Writer
}

// Result is the aggregate returned after the stream closes.
type Result struct {
	Counts   map[check.Status]int
	Total    int
	Duration time.Duration
}

// Record is the stable JSON shape.
type Record struct {
	Name       string        `json:"name"`
	Target     string        `json:"target"`
	Label      string        `json:"label"`
	Status     string        `json:"status"`
	Confidence string        `json:"confidence"`
	Query      string        `json:"query"`
	Summary    string        `json:"summary,omitempty"`
	URL        string        `json:"url,omitempty"`
	LatencyMS  int64         `json:"latency_ms"`
	Cached     bool          `json:"cached,omitempty"`
	Error      string        `json:"error,omitempty"`
	Fields     []check.Field `json:"fields,omitempty"`
}

// Run consumes engine events and writes them in the requested format.
func Run(events <-chan engine.Event, jobs []engine.Job, opts Options) (Result, error) {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Err == nil {
		opts.Err = io.Discard
	}
	if opts.Format == "" {
		opts.Format = "table"
	}
	if opts.Format != "table" && opts.Format != "json" && opts.Format != "ndjson" {
		return Result{}, fmt.Errorf("unknown format %q (use table, json or ndjson)", opts.Format)
	}

	res := Result{Counts: map[check.Status]int{}, Total: len(jobs)}
	start := time.Now()
	records := make([]Record, 0, len(jobs))

	table := newTableWriter(opts.Out, jobs, opts.NoColor, opts.ASCII)
	if opts.Format == "table" {
		table.header()
	}

	for ev := range events {
		if ev.Kind != engine.EventSettled {
			continue
		}
		res.Counts[ev.Result.Status]++
		if opts.OnlyAvailable && ev.Result.Status != check.StatusAvailable {
			continue
		}
		rec := recordOf(ev)
		switch opts.Format {
		case "table":
			table.row(ev)
		case "ndjson":
			if err := json.NewEncoder(opts.Out).Encode(rec); err != nil {
				return res, err
			}
		case "json":
			records = append(records, rec)
		}
	}
	res.Duration = time.Since(start)

	switch opts.Format {
	case "json":
		enc := json.NewEncoder(opts.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(records); err != nil {
			return res, err
		}
	}
	printSummary(opts.Err, res)
	return res, nil
}

func recordOf(ev engine.Event) Record {
	errText := ""
	if ev.Result.Err != nil {
		errText = ev.Result.Err.Error()
	}
	return Record{
		Name:       ev.Job.Raw,
		Target:     string(ev.Job.Target),
		Label:      ev.Job.Label,
		Status:     ev.Result.Status.String(),
		Confidence: ev.Result.Confidence.String(),
		Query:      ev.Result.Query,
		Summary:    ev.Result.Summary,
		URL:        ev.Result.SourceURL,
		LatencyMS:  ev.Result.Latency.Milliseconds(),
		Cached:     ev.Result.Cached,
		Error:      errText,
		Fields:     ev.Result.Fields,
	}
}

func printSummary(w io.Writer, r Result) {
	parts := []string{
		fmt.Sprintf("%d available", r.Counts[check.StatusAvailable]),
		fmt.Sprintf("%d taken", r.Counts[check.StatusTaken]),
		fmt.Sprintf("%d unknown", r.Counts[check.StatusUnknown]),
	}
	if n := r.Counts[check.StatusTimeout]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d timeout", n))
	}
	if n := r.Counts[check.StatusRateLimited]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d rate limited", n))
	}
	if n := r.Counts[check.StatusError]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d error", n))
	}
	if n := r.Counts[check.StatusSkipped] + r.Counts[check.StatusCanceled]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", n))
	}
	fmt.Fprintf(w, "\n%s · %s\n", strings.Join(parts, " · "), r.Duration.Round(10*time.Millisecond))
}

type tableWriter struct {
	w       io.Writer
	nameW   int
	targetW int
	noColor bool
	ascii   bool
}

func newTableWriter(w io.Writer, jobs []engine.Job, noColor, ascii bool) *tableWriter {
	nameW, targetW := 12, 6
	for _, j := range jobs {
		if n := len(j.Label); n > nameW {
			nameW = n
		}
		if n := len(j.TargetLabel); n > targetW {
			targetW = n
		}
	}
	if nameW > 42 {
		nameW = 42
	}
	return &tableWriter{w: w, nameW: nameW, targetW: targetW, noColor: noColor, ascii: ascii}
}

func (t *tableWriter) header() {
	fmt.Fprintf(t.w, "%-2s %-*s %-*s %-10s %8s  %s\n",
		"", t.targetW, "TARGET", t.nameW, "NAME", "STATUS", "LATENCY", "SOURCE")
}

func (t *tableWriter) row(ev engine.Event) {
	glyph, color := glyphFor(ev.Result.Status, t.ascii)
	reset := "\x1b[0m"
	if t.noColor {
		color = ""
		reset = ""
	}
	status := ev.Result.Status.String()
	if ev.Result.Cached {
		status += "*"
	}
	latency := ev.Result.Latency.Round(time.Millisecond).String()
	if ev.Result.Latency <= 0 {
		latency = "-"
	}
	target := textutil.Clean(ev.Job.TargetLabel)
	label := textutil.Clean(ev.Job.Label)
	source := textutil.Clean(ev.Result.SourceURL)
	fmt.Fprintf(t.w, "%s%s%s %-*s %-*s %-10s %8s  %s\n",
		color, glyph, reset, t.targetW, target, t.nameW, label, status, latency, source)
}

func glyphFor(s check.Status, ascii bool) (string, string) {
	if ascii {
		switch s {
		case check.StatusAvailable:
			return "+", "\x1b[32m"
		case check.StatusTaken:
			return "x", "\x1b[31m"
		case check.StatusUnknown:
			return "!", "\x1b[33m"
		case check.StatusTimeout:
			return "T", "\x1b[33m"
		case check.StatusRateLimited:
			return "R", "\x1b[33m"
		case check.StatusError:
			return "X", "\x1b[31m"
		case check.StatusSkipped, check.StatusCanceled:
			return "-", "\x1b[90m"
		default:
			return ".", "\x1b[90m"
		}
	}
	switch s {
	case check.StatusAvailable:
		return "✓", "\x1b[32m"
	case check.StatusTaken:
		return "●", "\x1b[31m"
	case check.StatusUnknown:
		return "!", "\x1b[33m"
	case check.StatusTimeout:
		return "◷", "\x1b[33m"
	case check.StatusRateLimited:
		return "↻", "\x1b[33m"
	case check.StatusError:
		return "✕", "\x1b[31m"
	case check.StatusSkipped, check.StatusCanceled:
		return "–", "\x1b[90m"
	default:
		return "◌", "\x1b[90m"
	}
}
