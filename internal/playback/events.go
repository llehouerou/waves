package playback

import "time"

// StateChange is emitted when playback state changes.
type StateChange struct {
	Previous State
	Current  State
}

// TrackStart is emitted each time a track begins playing: the service started
// the player on it, or the player started it by itself (a gapless self-start).
// A track started again (repeat-one, a duplicated queue entry, Play on the
// track playing) is a new start.
//
// NOT emitted by a resume from pause, a move of the queue position that starts
// nothing (Next, Previous, JumpTo while stopped; QueueMoveTo, QueueAdvance), or
// a track that fails to open.
//
// It is sent before the StateChange or TrackChange of the same command.
type TrackStart struct {
	Track Track
	Index int // its queue index when it started
}

// TrackChange is emitted when the queue position moves to another track.
//
// Emitted by:
//   - Play: when the track being played differs from the last played track
//   - Next/Previous/JumpTo: playing or not
//   - handleTrackFinished: when a track ends and advances automatically
//
// NOT emitted by:
//   - QueueMoveTo/QueueAdvance: navigation without playback does not emit
//   - Pause/Stop: state changes do not emit TrackChange
//   - a first play, or repeat-one starting the same track again
//
// It does not say that a track began playing: TrackStart does.
type TrackChange struct {
	Previous      *Track
	Current       *Track
	PreviousIndex int
	Index         int
}

// QueueChange is emitted when the queue contents change (a queue edit) or its
// position moves without a track starting (QueueMoveTo, QueueAdvance).
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

// ErrorEvent is emitted when an error occurs during playback.
type ErrorEvent struct {
	Operation string // e.g., "play", "seek"
	Path      string // track path if applicable
	Err       error
}
