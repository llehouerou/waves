package queuepanel

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/cursor"
)

// Model represents the queue panel state.
type Model struct {
	ui.Base
	queue    *playlist.PlayingQueue
	cursor   cursor.Cursor
	selected map[int]bool
}

// New creates a new queue panel model.
func New(queue *playlist.PlayingQueue) Model {
	return Model{
		queue:    queue,
		cursor:   cursor.New(0), // Tight scrolling (no margin)
		selected: make(map[int]bool),
	}
}

// Update handles messages for the queue panel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.IsFocused() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		// Handle wheel scroll
		if msg.Button == tea.MouseButtonWheelUp {
			m.moveCursor(-1)
			return m, nil
		}
		if msg.Button == tea.MouseButtonWheelDown {
			m.moveCursor(1)
			return m, nil
		}
		// Handle middle click (play track)
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonMiddle {
			if m.queue.Len() > 0 && m.cursor.Pos() < m.queue.Len() {
				m.clearSelection()
				idx := m.cursor.Pos()
				return m, func() tea.Msg {
					return ActionMsg(JumpToTrack{Index: idx})
				}
			}
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Handle common list navigation keys via cursor
		if m.cursor.HandleKey(key, m.queue.Len(), m.listHeight()) {
			return m, nil
		}

		switch key {
		case "x":
			// Toggle selection on current item
			if m.queue.Len() > 0 && m.cursor.Pos() < m.queue.Len() {
				if m.selected[m.cursor.Pos()] {
					delete(m.selected, m.cursor.Pos())
				} else {
					m.selected[m.cursor.Pos()] = true
				}
			}
		case "enter":
			if m.queue.Len() > 0 && m.cursor.Pos() < m.queue.Len() {
				m.clearSelection()
				idx := m.cursor.Pos()
				return m, func() tea.Msg {
					return ActionMsg(JumpToTrack{Index: idx})
				}
			}
		case "d", "delete":
			if m.queue.Len() > 0 {
				m.deleteSelected()
				return m, func() tea.Msg { return ActionMsg(QueueChanged{}) }
			}
		case "D":
			if m.queue.Len() > 0 && len(m.selected) > 0 {
				m.keepOnlySelected()
				return m, func() tea.Msg { return ActionMsg(QueueChanged{}) }
			}
		case "c":
			if m.queue.Len() > 0 {
				m.clearExceptPlaying()
				return m, func() tea.Msg { return ActionMsg(QueueChanged{}) }
			}
		case "esc":
			if len(m.selected) > 0 {
				m.clearSelection()
			}
		case "shift+j", "shift+down":
			if m.moveSelected(1) {
				return m, func() tea.Msg { return ActionMsg(QueueChanged{}) }
			}
		case "shift+k", "shift+up":
			if m.moveSelected(-1) {
				return m, func() tea.Msg { return ActionMsg(QueueChanged{}) }
			}
		case "f":
			// Toggle favorite for selected tracks or current track
			trackIDs := m.getSelectedTrackIDs()
			if len(trackIDs) > 0 {
				return m, func() tea.Msg {
					return ActionMsg(ToggleFavorite{TrackIDs: trackIDs})
				}
			}
		}
	}

	return m, nil
}

func (m Model) listHeight() int {
	return m.ListHeight(ui.PanelOverhead)
}

// getSelectedTrackIDs returns library track IDs for selected items, or the current item if none selected.
// Only returns IDs for tracks that have a library ID (not filesystem-only tracks).
func (m Model) getSelectedTrackIDs() []int64 {
	if len(m.selected) > 0 {
		return m.getTrackIDsFromIndices(m.selected)
	}
	return m.getTrackIDsFromIndices(map[int]bool{m.cursor.Pos(): true})
}

func (m Model) getTrackIDsFromIndices(indices map[int]bool) []int64 {
	trackIDs := make([]int64, 0, len(indices))
	for idx := range indices {
		if idx >= m.queue.Len() {
			continue
		}
		track := m.queue.Track(idx)
		if track == nil || track.ID == 0 {
			continue
		}
		trackIDs = append(trackIDs, track.ID)
	}
	return trackIDs
}
