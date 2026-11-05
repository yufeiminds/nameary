package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yufeiminds/nameary/internal/check"
	"github.com/yufeiminds/nameary/internal/textutil"
)

type exportRecord struct {
	Name       string        `json:"name"`
	Target     string        `json:"target"`
	Label      string        `json:"label"`
	Status     string        `json:"status"`
	Confidence string        `json:"confidence"`
	Query      string        `json:"query"`
	Summary    string        `json:"summary,omitempty"`
	URL        string        `json:"source_url,omitempty"`
	LatencyMS  int64         `json:"latency_ms"`
	Cached     bool          `json:"cached,omitempty"`
	Error      string        `json:"error,omitempty"`
	Fields     []check.Field `json:"fields,omitempty"`
}

func (m *model) records() []exportRecord {
	out := make([]exportRecord, 0, len(m.rows))
	for _, r := range m.rows {
		errText := ""
		if r.result.Err != nil {
			errText = r.result.Err.Error()
		}
		out = append(out, exportRecord{
			Name:       r.job.Raw,
			Target:     string(r.job.Target),
			Label:      r.job.Label,
			Status:     r.status.String(),
			Confidence: r.result.Confidence.String(),
			Query:      r.job.Query,
			Summary:    r.result.Summary,
			URL:        r.result.SourceURL,
			LatencyMS:  r.result.Latency.Milliseconds(),
			Cached:     r.result.Cached,
			Error:      errText,
			Fields:     r.result.Fields,
		})
	}
	return out
}

func (m *model) exportJSON() tea.Cmd {
	data, err := json.MarshalIndent(m.records(), "", "  ")
	if err != nil {
		return m.setToast("export failed: " + err.Error())
	}
	path, err := writeExport("nameary-"+safeFile(m.name)+"-*.json", data)
	if err != nil {
		return m.setToast("export failed: " + err.Error())
	}
	return m.setToast("wrote " + path)
}

func (m *model) exportMarkdown() tea.Cmd {
	var b strings.Builder
	fmt.Fprintf(&b, "# nameary: %s\n\n", textutil.Clean(m.name))
	fmt.Fprintf(&b, "Generated %s\n\n", time.Now().Format(time.RFC3339))
	b.WriteString("| Target | Name | Status | Confidence | Latency | Source |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, r := range m.rows {
		source := escMD(textutil.Clean(r.result.SourceURL))
		if source == "" {
			source = "—"
		}
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s | %s | %s |\n",
			escMD(textutil.Clean(r.job.TargetLabel)), escMD(textutil.Clean(r.job.Label)), r.status.String(),
			r.result.Confidence.String(), r.result.Latency.Round(time.Millisecond), source)
	}
	path, err := writeExport("nameary-"+safeFile(m.name)+"-*.md", []byte(b.String()))
	if err != nil {
		return m.setToast("export failed: " + err.Error())
	}
	return m.setToast("wrote " + path)
}

// writeExport creates a unique 0600 file instead of following a predictable
// path, avoiding symlink/clobber attacks in a shared working directory.
func writeExport(pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp(".", pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return filepath.Base(name), nil
}

func safeFile(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		out = "results"
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

func escMD(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
