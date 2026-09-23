package playback

import (
	"time"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/tags"
)

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

	// Queue navigation (starts playback if active)
	JumpTo(index int) error

	// Queue position control (without playback); each emits QueueChange
	QueueAdvance() *Track         // Advance queue position (respects modes), returns track
	QueueMoveTo(index int) *Track // Move queue position to index, returns track

	// Queue edits: each is one undo step and emits QueueChange
	AddTracks(tracks ...Track)
	ReplaceTracks(tracks ...Track) *Track // Returns track at index 0 or nil
	RemoveTracks(indices []int)
	MoveTracks(indices []int, delta int) // No-op if any track would leave the queue
	ClearQueue()

	// RestoreQueue puts back a saved queue at startup. Not a queue edit: no
	// undo step, no event, and nothing counts as played yet.
	RestoreQueue(saved SavedQueue)

	// State queries
	State() State
	IsPlaying() bool
	IsStopped() bool
	IsPaused() bool
	Position() time.Duration
	Duration() time.Duration
	CurrentTrack() *Track
	TrackInfo() *tags.FileInfo
	Player() player.Interface // Direct player access (for UI rendering)

	// Queue queries
	QueueTracks() []Track
	QueueCurrentIndex() int
	QueueLen() int
	QueueIsEmpty() bool
	QueueHasNext() bool
	QueuePeekNext() *Track

	// Queue history
	Undo() bool
	Redo() bool

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

// SavedQueue is a queue as persisted between runs.
type SavedQueue struct {
	Tracks     []Track
	Index      int // -1 if no track was current
	RepeatMode RepeatMode
	Shuffle    bool
}
