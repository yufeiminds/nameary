package tui

import (
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/harmonica"
)

type frameMsg time.Time

type animState struct {
	logoStart time.Time
	frame     int

	detailSpring harmonica.Spring
	detailPos    float64
	detailVel    float64
	detailTarget float64

	summaryStart time.Time
}

func newAnim() animState {
	return animState{detailSpring: harmonica.NewSpring(harmonica.FPS(60), 9.0, 1.0)}
}

func (m *model) tickIfNeeded() tea.Cmd {
	delay, ok := m.tickDelay()
	if !ok {
		return nil
	}
	return tea.Tick(delay, func(t time.Time) tea.Msg { return frameMsg(t) })
}

func (m *model) tickDelay() (time.Duration, bool) {
	now := m.now
	if now.IsZero() {
		now = time.Now()
	}
	if !m.toastUntil.IsZero() && now.Before(m.toastUntil) {
		return 200 * time.Millisecond, true
	}
	if m.opts.ReducedMotion {
		return 0, false
	}
	if !m.anim.logoStart.IsZero() && now.Sub(m.anim.logoStart) < 900*time.Millisecond {
		return time.Second / 60, true
	}
	if m.detail != nil && math.Abs(m.anim.detailPos-m.anim.detailTarget) > 0.4 {
		return time.Second / 60, true
	}
	if !m.anim.summaryStart.IsZero() && now.Sub(m.anim.summaryStart) < 400*time.Millisecond {
		return time.Second / 60, true
	}
	flash := false
	for _, r := range m.rows {
		if !r.settledAt.IsZero() && now.Sub(r.settledAt) < 420*time.Millisecond {
			flash = true
			break
		}
	}
	if flash {
		return time.Second / 60, true
	}
	if m.phase == phaseRunning && m.hasPending() {
		return time.Second / 30, true
	}
	return 0, false
}

func (m *model) onFrame(t time.Time) tea.Cmd {
	m.now = t
	m.anim.frame++
	if m.detail != nil {
		if math.Abs(m.anim.detailPos-m.anim.detailTarget) <= 0.4 {
			m.anim.detailPos = m.anim.detailTarget
			m.anim.detailVel = 0
		} else {
			m.anim.detailPos, m.anim.detailVel = m.anim.detailSpring.Update(
				m.anim.detailPos, m.anim.detailVel, m.anim.detailTarget)
		}
	}
	if !m.toastUntil.IsZero() && !t.Before(m.toastUntil) {
		m.toast = ""
		m.toastUntil = time.Time{}
	}
	return m.tickIfNeeded()
}

func (m *model) flashAmount(r *row) float64 {
	if m.opts.ReducedMotion || r.settledAt.IsZero() {
		return 0
	}
	elapsed := m.now.Sub(r.settledAt)
	if elapsed < 0 || elapsed >= 420*time.Millisecond {
		return 0
	}
	return 1 - float64(elapsed)/float64(420*time.Millisecond)
}

func (m *model) hasPending() bool {
	for _, r := range m.rows {
		if !r.status.IsFinal() {
			return true
		}
	}
	return false
}
