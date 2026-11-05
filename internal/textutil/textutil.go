// Package textutil contains output-safety helpers for terminal-facing text.
package textutil

import "strings"

// Clean removes control characters that could move the cursor, change colors,
// open OSC 8 links, or reorder text. It keeps printable Unicode and collapses
// newlines/tabs to spaces so each value stays on one terminal line.
func Clean(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f:
			// C0 controls, including ESC.
			continue
		case r >= 0x80 && r <= 0x9f:
			// C1 controls.
			continue
		case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
			// Bidirectional override/isolate controls.
			continue
		case r == 0x2028 || r == 0x2029:
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
