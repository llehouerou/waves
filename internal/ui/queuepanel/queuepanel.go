package queuepanel

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/list"
)

// Queue is the read side of the playback queue the panel shows. The panel
// never edits it: changes go out as RemoveTracks and MoveTracks actions.
type Queue interface {
	QueueTracks() []playback.Track
	QueueCurrentIndex() int
	QueueLen() int
	RepeatMode() playback.RepeatMode
	Shuffle() bool
}

// Model represents the queue panel state.
type Model struct {
	list      list.Model[struct{}] // Items managed externally by queue
	queue     Queue
	selected  map[int]bool
	favorites map[int64]bool
}

// New creates a new queue panel model.
func New(queue Queue) Model {
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
	result := m.list.Update(msg, m.queue.QueueLen())
	switch result.Action { //nolint:exhaustive // Only handling specific actions
	case list.ActionEnter, list.ActionMiddleClick:
		if m.queue.QueueLen() > 0 {
			m.clearSelection()
			return m, actionCmd(JumpToTrack{Index: result.Index})
		}
	case list.ActionDelete:
		if m.queue.QueueLen() > 0 {
			return m, actionCmd(m.deleteSelected())
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
		if m.queue.QueueLen() > 0 && len(m.selected) > 0 {
			return m, actionCmd(m.keepOnlySelected())
		}
	case "c":
		if m.queue.QueueLen() > 0 {
			return m, actionCmd(m.clearExceptPlaying())
		}
	case "esc":
		if len(m.selected) > 0 {
			m.clearSelection()
		}
	case "J", "shift+down": // Bubble Tea reports shift+j as the rune J
		if move, ok := m.moveSelected(1); ok {
			return m, actionCmd(move)
		}
	case "K", "shift+up":
		if move, ok := m.moveSelected(-1); ok {
			return m, actionCmd(move)
		}
	case "F":
		trackIDs := m.getSelectedTrackIDs()
		if len(trackIDs) > 0 {
			return m, actionCmd(ToggleFavorite{TrackIDs: trackIDs})
		}
	case "ctrl+a":
		trackIDs := m.getSelectedTrackIDs()
		if len(trackIDs) > 0 {
			return m, actionCmd(AddToPlaylist{TrackIDs: trackIDs})
		}
	case "L":
		tracks := m.queue.QueueTracks()
		if pos := m.list.Cursor().Pos(); pos < len(tracks) {
			track := tracks[pos]
			return m, actionCmd(GoToSource{
				TrackID: track.ID,
				Path:    track.Path,
				Album:   track.Album,
				Artist:  track.Artist,
			})
		}
	}
	return m, nil
}

// actionCmd returns a command that reports a queue panel action.
func actionCmd(a action.Action) tea.Cmd {
	return func() tea.Msg { return ActionMsg(a) }
}

// SetFavorites updates the favorites map for displaying favorite icons.
func (m *Model) SetFavorites(favorites map[int64]bool) {
	m.favorites = favorites
}

// isFavorite reports whether a queue track is a library favorite.
func (m Model) isFavorite(track playback.Track) bool {
	return track.ID != 0 && m.favorites[track.ID]
}

// toggleSelection toggles selection on the current item.
func (m *Model) toggleSelection() {
	pos := m.list.Cursor().Pos()
	if pos < m.queue.QueueLen() {
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
	tracks := m.queue.QueueTracks()
	trackIDs := make([]int64, 0, len(m.selected))
	for _, idx := range m.targetIndices() {
		if idx < len(tracks) && tracks[idx].ID != 0 {
			trackIDs = append(trackIDs, tracks[idx].ID)
		}
	}
	return trackIDs
}
