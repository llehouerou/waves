package queuepanel

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/ui"
)

// JumpToTrackMsg requests playback jump to a specific queue index.
// Root model should start playback at the specified index.
// Emitted when user presses Enter on a queue item.
type JumpToTrackMsg struct {
	Index int
}

// QueueChangedMsg is emitted when queue contents or order changes.
// Root model should persist queue state when received.
// Emitted on delete (d) and move (shift+j/k) operations.
type QueueChangedMsg struct{}

// Model represents the queue panel state.
type Model struct {
	queue    *playlist.PlayingQueue
	cursor   int
	offset   int
	width    int
	height   int
	focused  bool
	selected map[int]bool
}

// New creates a new queue panel model.
func New(queue *playlist.PlayingQueue) Model {
	return Model{
		queue:    queue,
		cursor:   0,
		offset:   0,
		selected: make(map[int]bool),
	}
}

// SetFocused sets whether the panel is focused.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether the panel is focused.
func (m Model) IsFocused() bool {
	return m.focused
}

// SetSize sets the panel dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages for the queue panel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if !m.focused {
		return m, nil
	}

	switch keyMsg.String() {
	case "x":
		// Toggle selection on current item
		if m.queue.Len() > 0 && m.cursor < m.queue.Len() {
			if m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = true
			}
		}
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "g":
		m.cursor = 0
		m.offset = 0
	case "G":
		if m.queue.Len() > 0 {
			m.cursor = m.queue.Len() - 1
			m.ensureCursorVisible()
		}
	case "enter":
		if m.queue.Len() > 0 && m.cursor < m.queue.Len() {
			m.clearSelection()
			return m, func() tea.Msg {
				return JumpToTrackMsg{Index: m.cursor}
			}
		}
	case "d", "delete":
		if m.queue.Len() > 0 {
			m.deleteSelected()
			return m, func() tea.Msg { return QueueChangedMsg{} }
		}
	case "D":
		if m.queue.Len() > 0 && len(m.selected) > 0 {
			m.keepOnlySelected()
			return m, func() tea.Msg { return QueueChangedMsg{} }
		}
	case "esc":
		if len(m.selected) > 0 {
			m.clearSelection()
		}
	case "shift+j", "shift+down":
		if m.moveSelected(1) {
			return m, func() tea.Msg { return QueueChangedMsg{} }
		}
	case "shift+k", "shift+up":
		if m.moveSelected(-1) {
			return m, func() tea.Msg { return QueueChangedMsg{} }
		}
	}

	return m, nil
}

func (m Model) listHeight() int {
	// Account for border + header + separator
	return m.height - ui.PanelOverhead
}
