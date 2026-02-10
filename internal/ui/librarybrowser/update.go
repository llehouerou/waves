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
	w1, w2, _ := m.columnWidths()
	borderOverhead := 2 // left + right border per column

	col := ColumnArtists
	x := msg.X
	if x >= w1+borderOverhead {
		col = ColumnAlbums
		x -= w1 + borderOverhead
	}
	if x >= w2+borderOverhead {
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
	// Each column renders: 2 (border) + 1 (title) + 1 (blank) + colHeight (items) = colHeight + 4
	// Description panel renders: 2 (border) + descriptionHeight (4) = 6
	// Total: colHeight + 4 + 6 = colHeight + 10 = m.height
	return max(m.height-10, 1)
}

// minColumnWidth is the minimum inner width for any column.
const minColumnWidth = 12

// columnWidths returns the inner width for each column (artists, albums, tracks).
// The active column gets 50% of available space, the other two get 25% each,
// subject to a minimum width. The last column absorbs any rounding remainder.
func (m Model) columnWidths() (w1, w2, w3 int) {
	available := m.width - 6 // 3 columns × 2 border chars
	if available < 3*minColumnWidth {
		// Not enough room for weighted split — distribute equally.
		base := max(available/3, 1)
		return base, base, max(available-2*base, 1)
	}

	half := available / 2
	quarter := available / 4

	switch m.activeColumn {
	case ColumnArtists:
		w1 = half
		w2 = quarter
	case ColumnAlbums:
		w1 = quarter
		w2 = half
	case ColumnTracks:
		w1 = quarter
		w2 = quarter
	}
	w3 = available - w1 - w2
	return w1, w2, w3
}
