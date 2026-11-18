package playlist

import (
	"math/rand/v2"
	"slices"
)

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
	// current is the entry the position is on. While the position is -1, it
	// is the entry a queue edit took out of the list while it was current,
	// for an undo or a redo that brings it back, until the position is set
	// to another track.
	current    EntryID
	repeatMode RepeatMode
	shuffle    bool
	history    *QueueHistory[entry]
}

// NewQueue creates a new empty playing queue.
func NewQueue() *PlayingQueue {
	q := &PlayingQueue{
		playlist:     NewPlaylist(),
		currentIndex: -1,
		history:      NewQueueHistory[entry](50),
	}
	q.record() // the empty queue is the oldest state undo returns to
	return q
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

// EntryAt returns the entry at index, or the zero EntryID if out of range.
func (q *PlayingQueue) EntryAt(index int) EntryID {
	return q.playlist.EntryAt(index)
}

// IndexOf returns where the queue holds the entry id, or -1 if it does not.
func (q *PlayingQueue) IndexOf(id EntryID) int {
	return q.playlist.IndexOf(id)
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
// Returns nil when no track is current (index -1), as HasNext and PeekNext do:
// a queue whose current track was removed does not resume at the first one.
func (q *PlayingQueue) Next() *Track {
	if q.playlist.Len() == 0 || q.currentIndex < 0 {
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
		return q.moveTo(newIdx)
	}

	// Normal next
	if q.currentIndex < q.playlist.Len()-1 {
		return q.moveTo(q.currentIndex + 1)
	}

	// At end of queue
	if q.repeatMode == RepeatAll {
		return q.moveTo(0)
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

// PeekNext returns the next track without advancing the queue.
// Returns nil if there is no next track.
func (q *PlayingQueue) PeekNext() *Track {
	if q.playlist.Len() == 0 || q.currentIndex < 0 {
		return nil
	}

	// Repeat One: next is current track
	if q.repeatMode == RepeatOne {
		return q.Current()
	}

	// Shuffle: can't predict next (would need to pick randomly)
	// Return nil to disable gapless in shuffle mode
	if q.shuffle {
		return nil
	}

	// Normal next
	if q.currentIndex < q.playlist.Len()-1 {
		return q.playlist.Track(q.currentIndex + 1)
	}

	// At end with repeat all
	if q.repeatMode == RepeatAll {
		return q.playlist.Track(0)
	}

	return nil
}

// JumpTo sets the current index to the specified position.
// Returns the track at that position, or nil if invalid.
func (q *PlayingQueue) JumpTo(index int) *Track {
	if index < 0 || index >= q.playlist.Len() {
		return nil
	}
	return q.moveTo(index)
}

// moveTo sets the position to the entry at index, in range, and returns its
// track.
func (q *PlayingQueue) moveTo(index int) *Track {
	q.currentIndex = index
	q.current = q.playlist.EntryAt(index)
	return q.Current()
}

// Add appends tracks to the queue without changing playback.
func (q *PlayingQueue) Add(tracks ...Track) {
	q.playlist.Add(tracks...)
	q.record()
}

// AddAndPlay appends tracks and jumps to the first added track.
// Returns the track to play.
func (q *PlayingQueue) AddAndPlay(tracks ...Track) *Track {
	if len(tracks) == 0 {
		return nil
	}
	insertIndex := q.playlist.Len()
	q.playlist.Add(tracks...)
	q.record()
	return q.moveTo(insertIndex)
}

// Replace clears the queue, adds tracks, and sets index to 0.
// Returns the first track to play. With no tracks, the queue is left with no
// current track, as Clear leaves it.
func (q *PlayingQueue) Replace(tracks ...Track) *Track {
	q.playlist.Clear()
	q.playlist.Add(tracks...)
	q.record()
	q.currentIndex = -1
	if len(tracks) == 0 {
		return nil
	}
	return q.moveTo(0)
}

// RemoveIndices removes the tracks at the given indices as one history entry.
// Out-of-range indices are ignored; returns false when nothing was removed.
// The position follows the current entry. If the current track is removed,
// currentIndex becomes -1 and playback stops when the current track finishes;
// an undo or a redo that brings that entry back makes it current again.
func (q *PlayingQueue) RemoveIndices(indices []int) bool {
	remove := make([]int, 0, len(indices))
	for _, idx := range indices {
		if idx >= 0 && idx < q.playlist.Len() && !slices.Contains(remove, idx) {
			remove = append(remove, idx)
		}
	}
	if len(remove) == 0 {
		return false
	}

	slices.Sort(remove)
	for _, idx := range slices.Backward(remove) {
		q.playlist.Remove(idx)
	}
	q.record()

	if slices.Contains(remove, q.currentIndex) {
		q.currentIndex = -1
	} else {
		q.currentIndex = IndexAfterRemove(q.currentIndex, remove)
	}
	return true
}

// IndexAfterRemove returns where the track at idx ends up once removed are
// taken out. A removed idx maps to the position of the track that followed it.
// Duplicate and negative entries in removed are ignored.
func IndexAfterRemove(idx int, removed []int) int {
	if idx < 0 {
		return idx
	}
	below := slices.DeleteFunc(slices.Clone(removed), func(r int) bool { return r < 0 || r >= idx })
	slices.Sort(below)
	return idx - len(slices.Compact(below))
}

// IndexAfterMove returns where the track at idx ends up once the tracks at
// indices have each moved by delta: moved tracks land at their index plus
// delta, the others keep their relative order in the remaining slots.
// The move must be in bounds (see MoveIndices).
func IndexAfterMove(idx int, indices []int, delta int) int {
	if idx < 0 {
		return idx
	}
	if slices.Contains(indices, idx) {
		return idx + delta
	}
	rank := IndexAfterRemove(idx, indices)
	for pos := 0; ; pos++ {
		if slices.ContainsFunc(indices, func(i int) bool { return i+delta == pos }) {
			continue
		}
		if rank == 0 {
			return pos
		}
		rank--
	}
}

// Clear removes all tracks and leaves the queue with no current track; an
// undo that brings the current entry back makes it current again.
func (q *PlayingQueue) Clear() {
	q.playlist.Clear()
	q.record()
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

// MoveIndices moves the tracks at indices by delta positions as one history
// entry. Returns false, moving nothing, if any track would leave the queue.
func (q *PlayingQueue) MoveIndices(indices []int, delta int) bool {
	if len(indices) == 0 || delta == 0 {
		return false
	}

	sorted := slices.Sorted(slices.Values(indices))
	if sorted[0]+delta < 0 || sorted[len(sorted)-1]+delta >= q.playlist.Len() {
		return false
	}

	// Move the tracks nearest the destination first, so each one lands in a
	// slot the others have already vacated.
	if delta > 0 {
		slices.Reverse(sorted)
	}
	for _, idx := range sorted {
		q.playlist.Move(idx, idx+delta)
	}
	q.record()
	q.currentIndex = IndexAfterMove(q.currentIndex, indices, delta)
	return true
}

// Undo restores the previous track list state. The position follows the
// current entry: it is where the restored list holds it, or -1 when the list
// does not hold it. An entry an edit took out while it was current is current
// again when the restored list brings it back, unless the position was set to
// another track since.
// Returns true if undo was performed.
func (q *PlayingQueue) Undo() bool {
	entries, ok := q.history.Undo()
	if !ok {
		return false
	}
	q.restoreEntries(entries)
	return true
}

// Redo restores the next track list state. The position follows the current
// entry, as for Undo.
// Returns true if redo was performed.
func (q *PlayingQueue) Redo() bool {
	entries, ok := q.history.Redo()
	if !ok {
		return false
	}
	q.restoreEntries(entries)
	return true
}

// restoreEntries puts back a recorded list and moves the position to where it
// holds the current entry, or to -1.
func (q *PlayingQueue) restoreEntries(entries []entry) {
	q.playlist.restore(entries)
	q.currentIndex = q.playlist.IndexOf(q.current)
}

// CanUndo returns true if there is a previous state to undo to.
func (q *PlayingQueue) CanUndo() bool {
	return q.history.CanUndo()
}

// CanRedo returns true if there is a next state to redo to.
func (q *PlayingQueue) CanRedo() bool {
	return q.history.CanRedo()
}

// ClearHistory forgets every earlier state: the current track list becomes
// the oldest one undo can return to.
func (q *PlayingQueue) ClearHistory() {
	q.history = NewQueueHistory[entry](q.history.maxSize)
	q.record()
}

// Restore replaces the queue with saved tracks, positioned on index (ignored
// when out of range). It is not an edit: the restored queue becomes the
// oldest state undo can return to.
func (q *PlayingQueue) Restore(tracks []Track, index int) {
	q.playlist.Clear()
	q.playlist.Add(tracks...)
	q.currentIndex = -1
	q.current = 0
	q.JumpTo(index)
	q.ClearHistory()
}

// record saves the list of entries an edit just produced. The history holds the
// state after each edit, so undo returns to the one before it.
func (q *PlayingQueue) record() {
	q.history.Push(q.playlist.snapshot())
}
