package librarybrowser

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	// Column switching
	switch key {
	case "h", "left":
		return m.moveLeft()
	case "l", "right":
		return m.moveRight()
	}

	// Vertical navigation in active column
	if m.handleVerticalNav(key) {
		return m, func() tea.Msg { return ActionMsg(NavigationChanged{}) }
	}

	return m, nil
}

func (m Model) moveLeft() (Model, tea.Cmd) {
	if m.activeColumn > ColumnArtists {
		m.activeColumn--
		return m, func() tea.Msg { return ActionMsg(NavigationChanged{}) }
	}
	return m, nil
}

func (m Model) moveRight() (Model, tea.Cmd) {
	if m.activeColumn < ColumnTracks {
		m.activeColumn++
		return m, func() tea.Msg { return ActionMsg(NavigationChanged{}) }
	}
	return m, nil
}

func (m *Model) handleVerticalNav(key string) bool {
	colHeight := m.columnHeight()

	switch m.activeColumn {
	case ColumnArtists:
		if m.artistCursor.HandleKey(key, len(m.artists), colHeight) {
			m.resetAlbumsAndTracks()
			return true
		}
	case ColumnAlbums:
		if m.albumCursor.HandleKey(key, len(m.albums), colHeight) {
			m.resetTracks()
			return true
		}
	case ColumnTracks:
		if m.trackCursor.HandleKey(key, len(m.tracks), colHeight) {
			return true
		}
	}
	return false
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	// Determine which column was clicked based on X position
	colWidth := m.columnWidth()
	borderOverhead := 2 // left + right border per column

	col := ColumnArtists
	x := msg.X
	if x >= colWidth+borderOverhead {
		col = ColumnAlbums
		x -= colWidth + borderOverhead
	}
	if x >= colWidth+borderOverhead {
		col = ColumnTracks
	}

	m.activeColumn = col
	colHeight := m.columnHeight()
	headerRows := 0 // lipgloss borders handle the header, mouse Y is relative to content

	switch m.activeColumn {
	case ColumnArtists:
		result, _ := m.artistCursor.HandleMouse(msg, len(m.artists), colHeight, headerRows)
		if result != 0 {
			m.resetAlbumsAndTracks()
			return m, func() tea.Msg { return ActionMsg(NavigationChanged{}) }
		}
	case ColumnAlbums:
		result, _ := m.albumCursor.HandleMouse(msg, len(m.albums), colHeight, headerRows)
		if result != 0 {
			m.resetTracks()
			return m, func() tea.Msg { return ActionMsg(NavigationChanged{}) }
		}
	case ColumnTracks:
		result, _ := m.trackCursor.HandleMouse(msg, len(m.tracks), colHeight, headerRows)
		if result != 0 {
			return m, func() tea.Msg { return ActionMsg(NavigationChanged{}) }
		}
	}

	return m, nil
}

// columnHeight returns the available height for list items in each column.
func (m Model) columnHeight() int {
	// Total height minus description panel (6 lines + 2 border) minus column borders (2)
	descHeight := 8
	return max(m.height-descHeight-2, 1)
}

// columnWidth returns the width of each column (excluding borders).
func (m Model) columnWidth() int {
	// 3 columns, each with 2 chars of border
	return max((m.width-6)/3, 1)
}
