package playback

import "time"

// StateChange is emitted when playback state changes.
type StateChange struct {
	Previous State
	Current  State
}

// TrackChange is emitted when the current track changes.
type TrackChange struct {
	Previous *Track
	Current  *Track
	Index    int
}

// QueueChange is emitted when the queue contents change.
type QueueChange struct {
	Tracks []Track
	Index  int
}

// ModeChange is emitted when repeat or shuffle mode changes.
type ModeChange struct {
	RepeatMode RepeatMode
	Shuffle    bool
}

// PositionChange is emitted when a seek occurs.
type PositionChange struct {
	Position time.Duration
}
