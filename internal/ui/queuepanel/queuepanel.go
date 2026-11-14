package queuepanel

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/list"
)

// keys resolves the panel's own keys against the queue bindings, so a key the
// panel acts on is a key the help lists.
var keys = keymap.NewResolver(keymap.ByContext(keymap.ContextQueue))

// Queue is the read side of the playback queue the panel shows. The panel
// never edits it: changes go out as RemoveTracks and MoveTracks actions.
type Queue interface {
	QueueTracks() []core.Track
	QueueCurrentIndex() int
	QueueLen() int
	RepeatMode() core.RepeatMode
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

// SetSize sets the component dimensions, and scrolls so the cursor stays in
// view when the panel shrinks (the player bar showing up as playback starts).
func (m *Model) SetSize(width, height int) {
	m.list.SetSize(width, height)
	m.keepCursorVisible()
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
		return m.handleAction(keys.Resolve(key.String()))
	}

	return m, nil
}

// handleAction runs the queue action a key resolves to.
func (m Model) handleAction(a keymap.Action) (Model, tea.Cmd) {
	switch a { //nolint:exhaustive // The list and the root handle the other actions
	case keymap.ActionToggleSelect:
		m.toggleSelection()
	case keymap.ActionKeepSelected:
		if m.queue.QueueLen() > 0 && len(m.selected) > 0 {
			return m, actionCmd(m.keepOnlySelected())
		}
	case keymap.ActionClear:
		if m.queue.QueueLen() > 0 {
			return m, actionCmd(m.clearExceptPlaying())
		}
	case keymap.ActionClearSelect:
		if len(m.selected) > 0 {
			m.clearSelection()
		}
	case keymap.ActionMoveItemDown:
		if move, ok := m.moveSelected(1); ok {
			return m, actionCmd(move)
		}
	case keymap.ActionMoveItemUp:
		if move, ok := m.moveSelected(-1); ok {
			return m, actionCmd(move)
		}
	case keymap.ActionToggleFavorite:
		trackIDs := m.getSelectedTrackIDs()
		if len(trackIDs) > 0 {
			return m, actionCmd(ToggleFavorite{TrackIDs: trackIDs})
		}
	case keymap.ActionAddToPlaylist:
		trackIDs := m.getSelectedTrackIDs()
		if len(trackIDs) > 0 {
			return m, actionCmd(AddToPlaylist{TrackIDs: trackIDs})
		}
	case keymap.ActionLocate:
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
func (m Model) isFavorite(track core.Track) bool {
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
