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
	PlayPath(path string) error // Play a track directly from a file path
	Pause() error
	Stop() error
	Toggle() error
	Next() error
	Previous() error
	Seek(delta time.Duration) error
	SeekTo(position time.Duration) error

	// Queue navigation (starts playback if active)
	JumpTo(index int) error

	// Queue position control (without playback)
	QueueAdvance() *Track         // Advance queue position (respects modes), returns track
	QueueMoveTo(index int) *Track // Move queue position to index, returns track

	// Queue manipulation
	AddTracks(tracks ...Track)
	ReplaceTracks(tracks ...Track) *Track // Returns track at index 0 or nil
	ClearQueue()

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
