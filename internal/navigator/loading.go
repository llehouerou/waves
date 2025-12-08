package navigator

// refresh reloads the current directory contents and updates parent/preview.
func (m *Model[T]) refresh() error {
	var err error
	m.currentItems, err = m.source.Children(m.current)
	if err != nil {
		return err
	}

	if m.cursor >= len(m.currentItems) {
		m.cursor = max(0, len(m.currentItems)-1)
	}

	m.updateParent()
	m.updatePreview()
	return nil
}

// updateParent loads the parent directory items and finds the cursor position.
func (m *Model[T]) updateParent() {
	parent := m.source.Parent(m.current)
	if parent == nil {
		m.parentItems = nil
		m.parentCursor = -1
		return
	}

	items, err := m.source.Children(*parent)
	if err != nil {
		m.parentItems = nil
		m.parentCursor = -1
		return
	}

	m.parentItems = items
	m.parentCursor = -1

	// Find the index of current in parent's children
	currentID := m.current.ID()
	for i, item := range items {
		if item.ID() == currentID {
			m.parentCursor = i
			break
		}
	}
}

// Refresh reloads the current directory contents.
func (m *Model[T]) Refresh() {
	_ = m.refresh()
}

// updatePreview loads the preview content for the selected item.
func (m *Model[T]) updatePreview() {
	m.previewItems = nil
	m.previewLines = nil

	if len(m.currentItems) == 0 || m.cursor >= len(m.currentItems) {
		return
	}

	selected := m.currentItems[m.cursor]

	// Check if the node provides width-aware preview lines
	if provider, ok := any(selected).(PreviewProviderWithWidth); ok {
		if lines := provider.PreviewLinesWithWidth(m.previewColumnWidth()); lines != nil {
			m.previewLines = lines
			return
		}
	}

	// Check if the node provides custom preview lines
	if provider, ok := any(selected).(PreviewProvider); ok {
		if lines := provider.PreviewLines(); lines != nil {
			m.previewLines = lines
			return
		}
	}

	// Default: show children for containers
	if selected.IsContainer() {
		items, err := m.source.Children(selected)
		if err != nil {
			return
		}
		m.previewItems = items
	}
}

// previewColumnWidth calculates the width of the preview column.
func (m *Model[T]) previewColumnWidth() int {
	if m.width == 0 {
		return 40 // reasonable default
	}
	innerWidth := m.width - 4        // border
	availableWidth := innerWidth - 2 // separators
	parentColWidth := availableWidth / 5
	currentColWidth := (availableWidth * 2) / 5
	return availableWidth - parentColWidth - currentColWidth
}
