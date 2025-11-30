package playlist

// PlayingQueue wraps a Playlist with playback state.
type PlayingQueue struct {
	playlist     *Playlist
	currentIndex int // -1 if nothing playing
}

// NewQueue creates a new empty playing queue.
func NewQueue() *PlayingQueue {
	return &PlayingQueue{
		playlist:     NewPlaylist(),
		currentIndex: -1,
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

// Next advances to the next track and returns it.
// Returns nil if there is no next track.
func (q *PlayingQueue) Next() *Track {
	if !q.HasNext() {
		return nil
	}
	q.currentIndex++
	return q.Current()
}

// HasNext returns true if there's a track after the current one.
func (q *PlayingQueue) HasNext() bool {
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
	q.playlist.Add(tracks...)
}

// AddAndPlay appends tracks and jumps to the first added track.
// Returns the track to play.
func (q *PlayingQueue) AddAndPlay(tracks ...Track) *Track {
	if len(tracks) == 0 {
		return nil
	}
	insertIndex := q.playlist.Len()
	q.playlist.Add(tracks...)
	q.currentIndex = insertIndex
	return q.Current()
}

// Replace clears the queue, adds tracks, and sets index to 0.
// Returns the first track to play.
func (q *PlayingQueue) Replace(tracks ...Track) *Track {
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
func (q *PlayingQueue) RemoveAt(index int) bool {
	if !q.playlist.Remove(index) {
		return false
	}

	// Adjust current index after removal
	if q.currentIndex > index {
		q.currentIndex--
	} else if q.currentIndex == index {
		// Removed current track - stay at same index (now points to next)
		// If we're past the end, clamp
		if q.currentIndex >= q.playlist.Len() {
			q.currentIndex = q.playlist.Len() - 1
		}
	}

	return true
}

// Clear removes all tracks and resets playback.
func (q *PlayingQueue) Clear() {
	q.playlist.Clear()
	q.currentIndex = -1
}

// Tracks returns all tracks in the queue.
func (q *PlayingQueue) Tracks() []Track {
	return q.playlist.Tracks()
}

// Len returns the number of tracks in the queue.
func (q *PlayingQueue) Len() int {
	return q.playlist.Len()
}

// IsEmpty returns true if the queue has no tracks.
func (q *PlayingQueue) IsEmpty() bool {
	return q.playlist.Len() == 0
}
