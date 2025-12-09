// internal/player/interface.go
package player

import "time"

// Interface defines the player contract for dependency injection and testing.
type Interface interface {
	Play(path string) error
	Stop()
	Pause()
	Resume()
	Toggle()
	State() State
	TrackInfo() *TrackInfo
	Position() time.Duration
	Duration() time.Duration
	Seek(delta time.Duration)
	OnFinished(fn func())
	FinishedChan() <-chan struct{}
	Done() <-chan struct{}
}

// Verify Player implements Interface at compile time.
var _ Interface = (*Player)(nil)
