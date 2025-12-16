package navigator

// focusNode moves the cursor to the node with the given ID.
func (m *Model[T]) focusNode(id string) {
	for i, node := range m.currentItems {
		if node.ID() == id {
			m.cursor.Jump(i, len(m.currentItems), m.listHeight())
			m.cursor.Center(len(m.currentItems), m.listHeight())
			m.updatePreview()
			return
		}
	}
	m.cursor.Reset()
	m.updatePreview()
}

// FocusByName selects the item with the given display name.
// If not found, selection stays at current position.
func (m *Model[T]) FocusByName(name string) {
	for i, node := range m.currentItems {
		if node.DisplayName() == name {
			m.cursor.Jump(i, len(m.currentItems), m.listHeight())
			m.cursor.Center(len(m.currentItems), m.listHeight())
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
			m.cursor.Jump(i, len(m.currentItems), m.listHeight())
			m.cursor.Center(len(m.currentItems), m.listHeight())
			m.updatePreview()
			return true
		}
	}
	return false
}

// Selected returns a pointer to the currently selected item, or nil if none.
func (m Model[T]) Selected() *T {
	if len(m.currentItems) == 0 || m.cursor.Pos() >= len(m.currentItems) {
		return nil
	}
	return &m.currentItems[m.cursor.Pos()]
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
