package navigator

import "github.com/llehouerou/waves/internal/ui"

// adjustOffset adjusts the scroll offset to keep the cursor visible with margin.
func (m *Model[T]) adjustOffset() {
	listHeight := m.height - ui.PanelOverhead
	if listHeight <= 0 {
		return
	}

	// Keep margin items above cursor when possible
	if m.cursor < m.offset+ui.ScrollMargin {
		m.offset = max(m.cursor-ui.ScrollMargin, 0)
	}

	// Keep margin items below cursor when possible
	if m.cursor >= m.offset+listHeight-ui.ScrollMargin {
		m.offset = m.cursor - listHeight + ui.ScrollMargin + 1
	}

	// Clamp offset to valid range
	maxOffset := max(len(m.currentItems)-listHeight, 0)
	m.offset = min(m.offset, maxOffset)
}

// centerCursor centers the view on the current cursor position.
func (m *Model[T]) centerCursor() {
	listHeight := m.height - ui.PanelOverhead
	if listHeight <= 0 {
		return
	}

	m.offset = max(m.cursor-listHeight/2, 0)
	maxOffset := max(len(m.currentItems)-listHeight, 0)
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

// focusNode moves the cursor to the node with the given ID.
func (m *Model[T]) focusNode(id string) {
	for i, node := range m.currentItems {
		if node.ID() == id {
			m.cursor = i
			m.centerCursor()
			m.updatePreview()
			return
		}
	}
	m.cursor = 0
	m.offset = 0
	m.updatePreview()
}

// FocusByName selects the item with the given display name.
// If not found, selection stays at current position.
func (m *Model[T]) FocusByName(name string) {
	for i, node := range m.currentItems {
		if node.DisplayName() == name {
			m.cursor = i
			m.centerCursor()
			m.updatePreview()
			return
		}
	}
}

// SelectByID selects the item with the given ID in the current view.
// Returns true if found, false otherwise. Does not navigate to other containers.
func (m *Model[T]) SelectByID(id string) bool {
	for i, node := range m.currentItems {
		if node.ID() == id {
			m.cursor = i
			m.centerCursor()
			m.updatePreview()
			return true
		}
	}
	return false
}

// Selected returns a pointer to the currently selected item, or nil if none.
func (m Model[T]) Selected() *T {
	if len(m.currentItems) == 0 || m.cursor >= len(m.currentItems) {
		return nil
	}
	return &m.currentItems[m.cursor]
}

// SelectedName returns the display name of the selected item, or empty if none.
func (m Model[T]) SelectedName() string {
	if selected := m.Selected(); selected != nil {
		return (*selected).DisplayName()
	}
	return ""
}

// SelectedID returns the ID of the selected item, or empty if none.
func (m Model[T]) SelectedID() string {
	if selected := m.Selected(); selected != nil {
		return (*selected).ID()
	}
	return ""
}
