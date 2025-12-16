package albumview

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
)

// QueueAlbumMsg requests queuing an album.
type QueueAlbumMsg struct {
	AlbumArtist string
	Album       string
	Replace     bool // true = replace queue, false = add
}

// Update handles messages for the album view.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureCursorVisible()
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey processes keyboard input.
func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	oldCursor := m.cursor

	switch msg.String() {
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "g", "home":
		m.cursor = 0
		m.ensureCursorInBounds()
	case "G", "end":
		m.cursor = len(m.flatList) - 1
		m.ensureCursorInBounds()
	case "ctrl+d":
		m.moveCursor(m.listHeight() / 2)
	case "ctrl+u":
		m.moveCursor(-m.listHeight() / 2)
	case "enter":
		if album := m.SelectedAlbum(); album != nil {
			return m, m.queueAlbumCmd(album, true)
		}
	case "a":
		if album := m.SelectedAlbum(); album != nil {
			return m, m.queueAlbumCmd(album, false)
		}
	}

	// Emit navigation changed if cursor moved
	if m.cursor != oldCursor {
		return m, m.navigationChangedCmd()
	}

	return m, nil
}

// navigationChangedCmd returns a command that emits NavigationChangedMsg.
func (m Model) navigationChangedCmd() tea.Cmd {
	return func() tea.Msg {
		return navigator.NavigationChangedMsg{}
	}
}

// handleMouse processes mouse input.
func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	oldCursor := m.cursor

	switch msg.Button { //nolint:exhaustive // Only handling wheel events
	case tea.MouseButtonWheelUp:
		m.moveCursor(-1)
	case tea.MouseButtonWheelDown:
		m.moveCursor(1)
	}

	// Emit navigation changed if cursor moved
	if m.cursor != oldCursor {
		return m, m.navigationChangedCmd()
	}

	return m, nil
}

// moveCursor moves cursor, skipping group headers.
func (m *Model) moveCursor(delta int) {
	if len(m.flatList) == 0 {
		return
	}

	newCursor := m.cursor + delta

	// Clamp to bounds
	newCursor = max(newCursor, 0)
	newCursor = min(newCursor, len(m.flatList)-1)

	// Skip headers when moving down
	if delta > 0 {
		for newCursor < len(m.flatList) && m.flatList[newCursor].IsHeader {
			newCursor++
		}
	}

	// Skip headers when moving up
	if delta < 0 {
		for newCursor >= 0 && m.flatList[newCursor].IsHeader {
			newCursor--
		}
	}

	// Final bounds check
	if newCursor >= 0 && newCursor < len(m.flatList) && !m.flatList[newCursor].IsHeader {
		m.cursor = newCursor
		m.ensureCursorVisible()
	}
}

func (m Model) queueAlbumCmd(album *library.AlbumEntry, replace bool) tea.Cmd {
	return func() tea.Msg {
		return QueueAlbumMsg{
			AlbumArtist: album.AlbumArtist,
			Album:       album.Album,
			Replace:     replace,
		}
	}
}
