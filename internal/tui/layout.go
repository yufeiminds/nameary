package tui

import (
	"sort"
	"strings"

	"github.com/yufeiminds/nameary/internal/check"
)

func (m *model) matches(r *row) bool {
	if m.onlyAvailable && r.status != check.StatusAvailable {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if q == "" {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{
		r.job.Label,
		r.job.TargetLabel,
		string(r.job.Category),
		r.job.Query,
		r.status.String(),
	}, " "))
	return strings.Contains(hay, q)
}

func (m *model) rebuildItems() {
	selected := -1
	if m.cursor >= 0 && m.cursor < len(m.items) && m.items[m.cursor].kind == itemRow {
		selected = m.items[m.cursor].row
	}

	var groups []check.Category
	byGroup := map[check.Category][]int{}
	for i, r := range m.rows {
		if _, ok := byGroup[r.job.Category]; !ok {
			groups = append(groups, r.job.Category)
		}
		byGroup[r.job.Category] = append(byGroup[r.job.Category], i)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return categoryRank(groups[i]) < categoryRank(groups[j])
	})

	var items []item
	for _, g := range groups {
		total, settled := 0, 0
		var visible []int
		for _, idx := range byGroup[g] {
			r := m.rows[idx]
			total++
			if r.status.IsFinal() {
				settled++
			}
			if m.matches(r) {
				visible = append(visible, idx)
			}
		}
		if len(visible) == 0 {
			continue
		}
		m.sortRows(visible)
		items = append(items, item{kind: itemHeader, group: g, total: total, settled: settled})
		for _, idx := range visible {
			items = append(items, item{kind: itemRow, group: g, row: idx})
		}
	}

	m.items = items
	cursor := -1
	if selected >= 0 {
		for i, it := range items {
			if it.kind == itemRow && it.row == selected {
				cursor = i
				break
			}
		}
	}
	if cursor < 0 {
		cursor = firstRow(items)
	}
	m.cursor = cursor
	m.clampOffset()
}

func (m *model) sortRows(rows []int) {
	switch m.sortMode {
	case sortStatus:
		sort.SliceStable(rows, func(a, b int) bool {
			return statusRank(m.rows[rows[a]].status) < statusRank(m.rows[rows[b]].status)
		})
	case sortLatency:
		sort.SliceStable(rows, func(a, b int) bool {
			ra, rb := m.rows[rows[a]], m.rows[rows[b]]
			if ra.status.IsFinal() != rb.status.IsFinal() {
				return ra.status.IsFinal()
			}
			return ra.result.Latency < rb.result.Latency
		})
	}
}

func categoryRank(c check.Category) int {
	switch c {
	case check.CategoryDomains:
		return 0
	case check.CategoryPackages:
		return 1
	case check.CategoryHandles:
		return 2
	default:
		return 3
	}
}

func statusRank(s check.Status) int {
	switch s {
	case check.StatusAvailable:
		return 0
	case check.StatusTaken:
		return 1
	case check.StatusUnknown:
		return 2
	case check.StatusChecking:
		return 3
	case check.StatusQueued:
		return 4
	case check.StatusError:
		return 5
	case check.StatusSkipped, check.StatusCanceled:
		return 6
	default:
		return 7
	}
}

func (m *model) listHeight() int {
	h := m.height
	h -= 6 // blank, brand, name, progress, stats, rule
	h -= m.footerHeight()
	if h < 1 {
		h = 1
	}
	return h
}

func (m *model) footerHeight() int {
	n := 2 // rule + keys
	if m.filterOn {
		n++
	}
	if m.toast != "" {
		n++
	}
	return n
}

func (m *model) clampOffset() {
	height := m.listHeight()
	if len(m.items) == 0 {
		m.cursor, m.offset = 0, 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+height {
		m.offset = m.cursor - height + 1
	}
	maxOffset := len(m.items) - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *model) move(delta int) {
	if len(m.items) == 0 {
		return
	}
	i := m.cursor + delta
	for i >= 0 && i < len(m.items) {
		if m.items[i].kind == itemRow {
			m.cursor = i
			m.clampOffset()
			return
		}
		i += delta
	}
}

func (m *model) moveTo(top bool) {
	if top {
		m.cursor = firstRow(m.items)
		m.offset = 0
		return
	}
	for i := len(m.items) - 1; i >= 0; i-- {
		if m.items[i].kind == itemRow {
			m.cursor = i
			m.clampOffset()
			return
		}
	}
}

func (m *model) page(delta int) {
	if len(m.items) == 0 {
		return
	}
	step := max(1, m.listHeight()-2)
	for n := 0; n < step; n++ {
		m.move(delta)
	}
}

func (m *model) visibleItems() []item {
	if len(m.items) == 0 {
		return nil
	}
	end := m.offset + m.listHeight()
	if end > len(m.items) {
		end = len(m.items)
	}
	return m.items[m.offset:end]
}

func (m *model) groupStats(g check.Category) (settled, total int) {
	for _, r := range m.rows {
		if r.job.Category != g {
			continue
		}
		total++
		if r.status.IsFinal() {
			settled++
		}
	}
	return settled, total
}
