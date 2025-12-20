package queuepanel

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/list"
)

// Model represents the queue panel state.
type Model struct {
	list      list.Model[struct{}] // Items managed externally by queue
	queue     *playlist.PlayingQueue
	selected  map[int]bool
	favorites map[int64]bool
}

// New creates a new queue panel model.
func New(queue *playlist.PlayingQueue) Model {
	return Model{
		list:     list.New[struct{}](2),
		queue:    queue,
		selected: make(map[int]bool),
	}
}

// SetFocused sets whether the component is focused.
func (m *Model) SetFocused(focused bool) {
	m.list.SetFocused(focused)
}

// IsFocused returns whether the component is focused.
func (m Model) IsFocused() bool {
	return m.list.IsFocused()
}

// SetSize sets the component dimensions.
func (m *Model) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

// Width returns the component width.
func (m Model) Width() int {
	return m.list.Width()
}

// Height returns the component height.
func (m Model) Height() int {
	return m.list.Height()
}

// Update handles messages for the queue panel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	// Delegate to list for common handling (navigation, enter, delete, mouse)
	result := m.list.Update(msg, m.queue.Len())
	switch result.Action { //nolint:exhaustive // Only handling specific actions
	case list.ActionEnter, list.ActionMiddleClick:
		if m.queue.Len() > 0 {
			m.clearSelection()
			return m, func() tea.Msg {
				return ActionMsg(JumpToTrack{Index: result.Index})
			}
		}
	case list.ActionDelete:
		if m.queue.Len() > 0 {
			m.deleteSelected()
			return m, func() tea.Msg { return ActionMsg(QueueChanged{}) }
		}
	}

	// Handle custom keys (only if focused)
	if key, ok := msg.(tea.KeyMsg); ok && m.IsFocused() {
		return m.handleCustomKey(key.String())
	}

	return m, nil
}

// handleCustomKey handles queue-specific key bindings.
func (m Model) handleCustomKey(key string) (Model, tea.Cmd) {
	switch key {
	case "x":
		m.toggleSelection()
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
	case "F":
		trackIDs := m.getSelectedTrackIDs()
		if len(trackIDs) > 0 {
			return m, func() tea.Msg {
				return ActionMsg(ToggleFavorite{TrackIDs: trackIDs})
			}
		}
	case "ctrl+a":
		trackIDs := m.getSelectedTrackIDs()
		if len(trackIDs) > 0 {
			return m, func() tea.Msg {
				return ActionMsg(AddToPlaylist{TrackIDs: trackIDs})
			}
		}
	case "g":
		pos := m.list.Cursor().Pos()
		if pos < m.queue.Len() {
			track := m.queue.Track(pos)
			if track != nil {
				return m, func() tea.Msg {
					return ActionMsg(GoToSource{
						TrackID: track.ID,
						Path:    track.Path,
						Album:   track.Album,
						Artist:  track.Artist,
					})
				}
			}
		}
	}
	return m, nil
}

// SetFavorites updates the favorites map for displaying favorite icons.
func (m *Model) SetFavorites(favorites map[int64]bool) {
	m.favorites = favorites
}

// isFavorite checks if a track at the given index is a favorite.
func (m Model) isFavorite(idx int) bool {
	if idx >= m.queue.Len() {
		return false
	}
	track := m.queue.Track(idx)
	if track == nil || track.ID == 0 {
		return false
	}
	return m.favorites[track.ID]
}

// toggleSelection toggles selection on the current item.
func (m *Model) toggleSelection() {
	pos := m.list.Cursor().Pos()
	if m.queue.Len() > 0 && pos < m.queue.Len() {
		if m.selected[pos] {
			delete(m.selected, pos)
		} else {
			m.selected[pos] = true
		}
	}
}

func (m Model) listHeight() int {
	return m.list.ListHeight(ui.PanelOverhead)
}

// getSelectedTrackIDs returns library track IDs for selected items, or the current item if none selected.
// Only returns IDs for tracks that have a library ID (not filesystem-only tracks).
func (m Model) getSelectedTrackIDs() []int64 {
	if len(m.selected) > 0 {
		return m.getTrackIDsFromIndices(m.selected)
	}
	return m.getTrackIDsFromIndices(map[int]bool{m.list.Cursor().Pos(): true})
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
