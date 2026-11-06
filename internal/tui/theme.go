package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/yufeiminds/nameary/internal/check"
)

type theme struct {
	dark    bool
	noColor bool
	ascii   bool

	text   color.Color
	muted  color.Color
	dim    color.Color
	accent color.Color
	green  color.Color
	rose   color.Color
	amber  color.Color
	red    color.Color
	indigo color.Color
}

func newTheme(noColor, ascii bool) theme {
	t := theme{dark: true, noColor: noColor, ascii: ascii}
	t.apply()
	return t
}

func (t *theme) setDark(dark bool) {
	t.dark = dark
	t.apply()
}

func (t *theme) apply() {
	if t.dark {
		t.text = lipgloss.Color("#E6EDF3")
		t.muted = lipgloss.Color("#7D8FA3")
		t.dim = lipgloss.Color("#46586A")
		t.accent = lipgloss.Color("#5EEAD4")
		t.green = lipgloss.Color("#34D399")
		t.rose = lipgloss.Color("#FB7185")
		t.amber = lipgloss.Color("#FBBF24")
		t.red = lipgloss.Color("#F87171")
		t.indigo = lipgloss.Color("#818CF8")
		return
	}
	t.text = lipgloss.Color("#1F2937")
	t.muted = lipgloss.Color("#64748B")
	t.dim = lipgloss.Color("#94A3B8")
	t.accent = lipgloss.Color("#0D9488")
	t.green = lipgloss.Color("#059669")
	t.rose = lipgloss.Color("#E11D48")
	t.amber = lipgloss.Color("#D97706")
	t.red = lipgloss.Color("#DC2626")
	t.indigo = lipgloss.Color("#4F46E5")
}

func (t theme) style(c color.Color) lipgloss.Style {
	if t.noColor || c == nil {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(c)
}

func (t theme) bold(c color.Color) lipgloss.Style {
	s := t.style(c)
	if !t.noColor {
		s = s.Bold(true)
	}
	return s
}

func (t theme) statusColor(s check.Status) color.Color {
	if t.noColor {
		return nil
	}
	switch s {
	case check.StatusAvailable:
		return t.green
	case check.StatusTaken:
		return t.rose
	case check.StatusUnknown, check.StatusTimeout, check.StatusRateLimited:
		return t.amber
	case check.StatusError:
		return t.red
	case check.StatusChecking:
		return t.accent
	case check.StatusSkipped, check.StatusCanceled:
		return t.dim
	default:
		return t.muted
	}
}

func (t theme) glyph(s check.Status, spinnerFrame string) string {
	if s == check.StatusChecking {
		if spinnerFrame != "" {
			return spinnerFrame
		}
		if t.ascii {
			return "o"
		}
		return "◐"
	}
	if t.ascii {
		switch s {
		case check.StatusAvailable:
			return "+"
		case check.StatusTaken:
			return "x"
		case check.StatusUnknown:
			return "!"
		case check.StatusTimeout:
			return "T"
		case check.StatusRateLimited:
			return "R"
		case check.StatusError:
			return "X"
		case check.StatusSkipped, check.StatusCanceled:
			return "-"
		default:
			return "."
		}
	}
	switch s {
	case check.StatusAvailable:
		return "✓"
	case check.StatusTaken:
		return "●"
	case check.StatusUnknown:
		return "!"
	case check.StatusTimeout:
		return "◷"
	case check.StatusRateLimited:
		return "↻"
	case check.StatusError:
		return "✕"
	case check.StatusSkipped, check.StatusCanceled:
		return "–"
	default:
		return "◌"
	}
}

func (t theme) flashColor(s check.Status) color.Color {
	if t.noColor {
		return nil
	}
	if t.dark {
		switch s {
		case check.StatusAvailable:
			return lipgloss.Color("#123A32")
		case check.StatusTaken:
			return lipgloss.Color("#3A1722")
		case check.StatusUnknown, check.StatusError, check.StatusTimeout, check.StatusRateLimited:
			return lipgloss.Color("#3A2C12")
		default:
			return lipgloss.Color("#1B2A3A")
		}
	}
	switch s {
	case check.StatusAvailable:
		return lipgloss.Color("#D1FAE5")
	case check.StatusTaken:
		return lipgloss.Color("#FFE4E6")
	case check.StatusUnknown, check.StatusError, check.StatusTimeout, check.StatusRateLimited:
		return lipgloss.Color("#FEF3C7")
	default:
		return lipgloss.Color("#E2E8F0")
	}
}

func (t theme) borderStyle() lipgloss.Style {
	return t.style(t.dim)
}

func (t theme) ellipsis() string {
	if t.ascii {
		return "..."
	}
	return "…"
}
