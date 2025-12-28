package playback

import "time"

// Service defines the playback service contract.
type Service interface {
	// Playback control
	Play() error
	Pause() error
	Stop() error
	Toggle() error
	Next() error
	Previous() error
	Seek(delta time.Duration) error
	SeekTo(position time.Duration) error

	// Queue navigation
	JumpTo(index int) error

	// State queries
	State() State
	Position() time.Duration
	Duration() time.Duration
	CurrentTrack() *Track
	Queue() []Track
	QueueIndex() int

	// Mode control
	RepeatMode() RepeatMode
	SetRepeatMode(mode RepeatMode)
	CycleRepeatMode() RepeatMode
	Shuffle() bool
	SetShuffle(enabled bool)
	ToggleShuffle() bool

	// Event subscription
	Subscribe() *Subscription

	// Lifecycle
	Close() error
}

// Subscription allows receiving playback events.
// TODO: Full implementation in Task 4.
type Subscription struct{}
