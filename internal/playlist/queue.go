package playlist

import "math/rand/v2"

// RepeatMode defines the repeat behavior for the queue.
type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatAll
	RepeatOne
	RepeatRadio
)

// PlayingQueue wraps a Playlist with playback state.
type PlayingQueue struct {
	playlist     *Playlist
	currentIndex int // -1 if nothing playing
	repeatMode   RepeatMode
	shuffle      bool
	history      *QueueHistory
}

// NewQueue creates a new empty playing queue.
func NewQueue() *PlayingQueue {
	return &PlayingQueue{
		playlist:     NewPlaylist(),
		currentIndex: -1,
		history:      NewQueueHistory(50),
	}
}

// Current returns the currently playing track, or nil if none.
func (q *PlayingQueue) Current() *Track {
	if q.currentIndex < 0 || q.currentIndex >= q.playlist.Len() {
		return nil
	}
	return q.playlist.Track(q.currentIndex)
}

// CurrentIndex returns the index of the currently playing track (-1 if none).
func (q *PlayingQueue) CurrentIndex() int {
	return q.currentIndex
}

// RepeatMode returns the current repeat mode.
func (q *PlayingQueue) RepeatMode() RepeatMode {
	return q.repeatMode
}

// SetRepeatMode sets the repeat mode.
func (q *PlayingQueue) SetRepeatMode(mode RepeatMode) {
	q.repeatMode = mode
}

// CycleRepeatMode cycles through repeat modes: Off -> All -> One -> Radio -> Off.
func (q *PlayingQueue) CycleRepeatMode() RepeatMode {
	q.repeatMode = (q.repeatMode + 1) % 4
	return q.repeatMode
}

// Shuffle returns whether shuffle is enabled.
func (q *PlayingQueue) Shuffle() bool {
	return q.shuffle
}

// SetShuffle sets the shuffle state.
func (q *PlayingQueue) SetShuffle(enabled bool) {
	q.shuffle = enabled
}

// ToggleShuffle toggles shuffle on/off and returns the new state.
func (q *PlayingQueue) ToggleShuffle() bool {
	q.shuffle = !q.shuffle
	return q.shuffle
}

// Next advances to the next track and returns it.
// Respects repeat mode and shuffle settings.
// Returns nil if there is no next track (and repeat is off).
func (q *PlayingQueue) Next() *Track {
	if q.playlist.Len() == 0 {
		return nil
	}

	// Repeat One: stay on current track
	if q.repeatMode == RepeatOne {
		return q.Current()
	}

	// Shuffle: pick random track (different from current if possible)
	if q.shuffle {
		if q.playlist.Len() == 1 {
			return q.Current()
		}
		newIdx := rand.IntN(q.playlist.Len() - 1) //nolint:gosec // shuffle doesn't need crypto-secure random
		if newIdx >= q.currentIndex {
			newIdx++
		}
		q.currentIndex = newIdx
		return q.Current()
	}

	// Normal next
	if q.currentIndex < q.playlist.Len()-1 {
		q.currentIndex++
		return q.Current()
	}

	// At end of queue
	if q.repeatMode == RepeatAll {
		q.currentIndex = 0
		return q.Current()
	}

	return nil
}

// HasNext returns true if there's a next track to play.
// Takes repeat mode and shuffle into account.
// Returns false if currentIndex is -1 (no current track in queue).
func (q *PlayingQueue) HasNext() bool {
	if q.playlist.Len() == 0 || q.currentIndex < 0 {
		return false
	}
	if q.repeatMode == RepeatOne || q.repeatMode == RepeatAll || q.shuffle {
		return true
	}
	return q.currentIndex < q.playlist.Len()-1
}

// JumpTo sets the current index to the specified position.
// Returns the track at that position, or nil if invalid.
func (q *PlayingQueue) JumpTo(index int) *Track {
	if index < 0 || index >= q.playlist.Len() {
		return nil
	}
	q.currentIndex = index
	return q.Current()
}

// Add appends tracks to the queue without changing playback.
func (q *PlayingQueue) Add(tracks ...Track) {
	q.history.Push(q.playlist.Tracks())
	q.playlist.Add(tracks...)
}

// AddAndPlay appends tracks and jumps to the first added track.
// Returns the track to play.
func (q *PlayingQueue) AddAndPlay(tracks ...Track) *Track {
	if len(tracks) == 0 {
		return nil
	}
	q.history.Push(q.playlist.Tracks())
	insertIndex := q.playlist.Len()
	q.playlist.Add(tracks...)
	q.currentIndex = insertIndex
	return q.Current()
}

// Replace clears the queue, adds tracks, and sets index to 0.
// Returns the first track to play.
func (q *PlayingQueue) Replace(tracks ...Track) *Track {
	q.history.Push(q.playlist.Tracks())
	q.playlist.Clear()
	q.currentIndex = -1
	if len(tracks) == 0 {
		return nil
	}
	q.playlist.Add(tracks...)
	q.currentIndex = 0
	return q.Current()
}

// RemoveAt removes the track at the given index.
// Adjusts currentIndex if necessary.
// If the currently playing track is removed, currentIndex becomes -1
// and playback will stop when the current track finishes.
func (q *PlayingQueue) RemoveAt(index int) bool {
	if index < 0 || index >= q.playlist.Len() {
		return false
	}
	q.history.Push(q.playlist.Tracks())
	q.playlist.Remove(index)

	// Adjust current index after removal
	if q.currentIndex > index {
		q.currentIndex--
	} else if q.currentIndex == index {
		// Removed current track - set to -1 so playback stops after current track
		q.currentIndex = -1
	}

	return true
}

// Clear removes all tracks and resets playback.
func (q *PlayingQueue) Clear() {
	q.history.Push(q.playlist.Tracks())
	q.playlist.Clear()
	q.currentIndex = -1
}

// Tracks returns all tracks in the queue.
func (q *PlayingQueue) Tracks() []Track {
	return q.playlist.Tracks()
}

// Track returns the track at the given index, or nil if out of bounds.
func (q *PlayingQueue) Track(index int) *Track {
	return q.playlist.Track(index)
}

// Len returns the number of tracks in the queue.
func (q *PlayingQueue) Len() int {
	return q.playlist.Len()
}

// IsEmpty returns true if the queue has no tracks.
func (q *PlayingQueue) IsEmpty() bool {
	return q.playlist.Len() == 0
}

// MoveIndices moves a set of indices by delta positions.
// Returns the new indices after the move, and whether the move was successful.
// If any selected item would go out of bounds, no move is performed.
func (q *PlayingQueue) MoveIndices(indices []int, delta int) ([]int, bool) {
	if len(indices) == 0 || delta == 0 {
		return indices, false
	}

	// Sort indices
	sorted := make([]int, len(indices))
	copy(sorted, indices)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Check bounds
	if delta < 0 {
		// Moving up: check if first selected item can move
		if sorted[0]+delta < 0 {
			return indices, false
		}
	} else {
		// Moving down: check if last selected item can move
		if sorted[len(sorted)-1]+delta >= q.playlist.Len() {
			return indices, false
		}
	}

	// Create a map of which indices are selected
	selectedSet := make(map[int]bool)
	for _, idx := range sorted {
		selectedSet[idx] = true
	}

	q.history.Push(q.playlist.Tracks())

	// Perform the moves
	if delta < 0 {
		q.moveIndicesUp(sorted, delta)
	} else {
		q.moveIndicesDown(sorted, delta)
	}

	// Calculate new indices
	newIndices := make([]int, len(indices))
	for i, idx := range indices {
		newIndices[i] = idx + delta
	}

	return newIndices, true
}

// moveIndicesUp moves sorted indices up (delta < 0).
func (q *PlayingQueue) moveIndicesUp(sorted []int, delta int) {
	for _, idx := range sorted {
		q.playlist.Move(idx, idx+delta)
		// Adjust currentIndex if needed
		if q.currentIndex == idx {
			q.currentIndex = idx + delta
		} else if q.currentIndex >= idx+delta && q.currentIndex < idx {
			q.currentIndex++
		}
	}
}

// moveIndicesDown moves sorted indices down (delta > 0).
func (q *PlayingQueue) moveIndicesDown(sorted []int, delta int) {
	for i := len(sorted) - 1; i >= 0; i-- {
		idx := sorted[i]
		q.playlist.Move(idx, idx+delta)
		// Adjust currentIndex if needed
		if q.currentIndex == idx {
			q.currentIndex = idx + delta
		} else if q.currentIndex > idx && q.currentIndex <= idx+delta {
			q.currentIndex--
		}
	}
}

// Undo restores the previous track list state.
// Returns true if undo was performed.
func (q *PlayingQueue) Undo() bool {
	tracks, ok := q.history.Undo()
	if !ok {
		return false
	}
	q.restoreTracks(tracks)
	return true
}

// Redo restores the next track list state.
// Returns true if redo was performed.
func (q *PlayingQueue) Redo() bool {
	tracks, ok := q.history.Redo()
	if !ok {
		return false
	}
	q.restoreTracks(tracks)
	return true
}

// restoreTracks replaces the playlist with the given tracks,
// clamping currentIndex to valid bounds.
func (q *PlayingQueue) restoreTracks(tracks []Track) {
	q.playlist.Clear()
	q.playlist.Add(tracks...)
	// Clamp currentIndex to valid bounds
	if q.currentIndex >= q.playlist.Len() {
		q.currentIndex = q.playlist.Len() - 1
	}
}

// CanUndo returns true if there is a previous state to undo to.
func (q *PlayingQueue) CanUndo() bool {
	return q.history.CanUndo()
}

// CanRedo returns true if there is a next state to redo to.
func (q *PlayingQueue) CanRedo() bool {
	return q.history.CanRedo()
}

// ClearHistory removes all history entries.
func (q *PlayingQueue) ClearHistory() {
	q.history = NewQueueHistory(q.history.maxSize)
}

// AddWithoutHistory appends tracks without creating a history entry.
// Use for bulk loading (e.g., restoring persisted state).
func (q *PlayingQueue) AddWithoutHistory(tracks ...Track) {
	q.playlist.Add(tracks...)
}

// SaveToHistory saves the current track list as a history entry.
// Use after bulk loading to establish the initial undoable state.
func (q *PlayingQueue) SaveToHistory() {
	q.history.Push(q.playlist.Tracks())
}
