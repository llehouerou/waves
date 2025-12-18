package albumview

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
)

// Update handles messages for the album view.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.IsFocused() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
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
	oldCursor := m.cursor.Pos()

	switch msg.String() {
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "g", "home":
		m.cursor.SetPos(0)
		m.ensureCursorInBounds()
	case "G", "end":
		m.cursor.SetPos(len(m.flatList) - 1)
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
	if m.cursor.Pos() != oldCursor {
		return m, m.navigationChangedCmd()
	}

	return m, nil
}

// navigationChangedCmd returns a command that emits NavigationChanged action.
func (m Model) navigationChangedCmd() tea.Cmd {
	return func() tea.Msg {
		return navigator.ActionMsg(navigator.NavigationChanged{})
	}
}

// handleMouse processes mouse input.
func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	oldCursor := m.cursor.Pos()

	switch msg.Button { //nolint:exhaustive // Only handling specific mouse events
	case tea.MouseButtonWheelUp:
		m.moveCursor(-1)
	case tea.MouseButtonWheelDown:
		m.moveCursor(1)
	case tea.MouseButtonMiddle:
		// Middle click: queue and play selected album (same as Enter)
		if msg.Action == tea.MouseActionPress {
			if album := m.SelectedAlbum(); album != nil {
				return m, m.queueAlbumCmd(album, true)
			}
		}
		return m, nil
	}

	// Emit navigation changed if cursor moved
	if m.cursor.Pos() != oldCursor {
		return m, m.navigationChangedCmd()
	}

	return m, nil
}

// moveCursor moves cursor, skipping group headers.
// This has header-skipping logic that cannot be delegated to the cursor package.
func (m *Model) moveCursor(delta int) {
	if len(m.flatList) == 0 {
		return
	}

	newPos := m.cursor.Pos() + delta

	// Clamp to bounds
	newPos = max(newPos, 0)
	newPos = min(newPos, len(m.flatList)-1)

	// Skip headers when moving down
	if delta > 0 {
		for newPos < len(m.flatList) && m.flatList[newPos].IsHeader {
			newPos++
		}
	}

	// Skip headers when moving up
	if delta < 0 {
		for newPos >= 0 && m.flatList[newPos].IsHeader {
			newPos--
		}
	}

	// Final bounds check
	if newPos >= 0 && newPos < len(m.flatList) && !m.flatList[newPos].IsHeader {
		m.cursor.SetPos(newPos)
		m.ensureCursorVisible()
	}
}

func (m Model) queueAlbumCmd(album *library.AlbumEntry, replace bool) tea.Cmd {
	return func() tea.Msg {
		return ActionMsg(QueueAlbum{
			AlbumArtist: album.AlbumArtist,
			Album:       album.Album,
			Replace:     replace,
		})
	}
}
