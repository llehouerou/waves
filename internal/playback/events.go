package playback

import "time"

// StateChange is emitted when playback state changes.
type StateChange struct {
	Previous State
	Current  State
}

// TrackChange is emitted when playback starts on a different track.
//
// Emitted by:
//   - Play: when the track being played differs from the last played track
//   - Next/Previous/JumpTo: when navigating with playback control
//   - handleTrackFinished: when a track ends and advances automatically
//
// NOT emitted by:
//   - QueueMoveTo/QueueAdvance: navigation without playback does not emit
//   - Pause/Stop: state changes do not emit TrackChange
//
// This design ensures that rapid navigation (debouncing) does not trigger
// multiple notifications - only when Play() is called does TrackChange fire.
//
// The app should handle all track-related side effects (notifications,
// scrobble, album art, lyrics) in response to this event.
type TrackChange struct {
	Previous      *Track
	Current       *Track
	PreviousIndex int
	Index         int
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

// ErrorEvent is emitted when an error occurs during playback.
type ErrorEvent struct {
	Operation string // e.g., "play", "seek"
	Path      string // track path if applicable
	Err       error
}
