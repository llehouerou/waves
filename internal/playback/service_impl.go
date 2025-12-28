// internal/playback/service_impl.go
package playback

import (
	"errors"
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

// Errors returned by playback service methods.
var (
	ErrEmptyQueue     = errors.New("queue is empty")
	ErrNoCurrentTrack = errors.New("no current track")
)

// Verify serviceImpl implements Service at compile time.
var _ Service = (*serviceImpl)(nil)

type serviceImpl struct {
	mu sync.RWMutex

	player player.Interface
	queue  *playlist.PlayingQueue

	subs   []*Subscription
	subsMu sync.RWMutex

	done   chan struct{}
	closed bool
}

// New creates a new playback service.
func New(p player.Interface, q *playlist.PlayingQueue) Service {
	s := &serviceImpl{
		player: p,
		queue:  q,
		done:   make(chan struct{}),
	}
	return s
}

// State returns the current playback state.
func (s *serviceImpl) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.playerStateToState(s.player.State())
}

func (s *serviceImpl) playerStateToState(ps player.State) State {
	switch ps {
	case player.Playing:
		return StatePlaying
	case player.Paused:
		return StatePaused
	case player.Stopped:
		return StateStopped
	default:
		return StateStopped
	}
}

// Position returns the current playback position.
func (s *serviceImpl) Position() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.player.Position()
}

// Duration returns the current track duration.
func (s *serviceImpl) Duration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.player.Duration()
}

// CurrentTrack returns the current track, or nil if none.
func (s *serviceImpl) CurrentTrack() *Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTrackLocked()
}

func (s *serviceImpl) currentTrackLocked() *Track {
	t := s.queue.Current()
	if t == nil {
		return nil
	}
	return &Track{
		ID:          t.ID,
		Path:        t.Path,
		Title:       t.Title,
		Artist:      t.Artist,
		Album:       t.Album,
		TrackNumber: t.TrackNumber,
		Duration:    t.Duration,
	}
}

// Queue returns a copy of all tracks in the queue.
func (s *serviceImpl) Queue() []Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tracks := s.queue.Tracks()
	result := make([]Track, len(tracks))
	for i, t := range tracks {
		result[i] = Track{
			ID:          t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
			Duration:    t.Duration,
		}
	}
	return result
}

// QueueIndex returns the current queue index (-1 if none).
func (s *serviceImpl) QueueIndex() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queue.CurrentIndex()
}

// RepeatMode returns the current repeat mode.
func (s *serviceImpl) RepeatMode() RepeatMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return RepeatMode(s.queue.RepeatMode())
}

// Shuffle returns whether shuffle is enabled.
func (s *serviceImpl) Shuffle() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queue.Shuffle()
}

// Subscribe creates a new event subscription.
func (s *serviceImpl) Subscribe() *Subscription {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	sub := newSubscription()
	s.subs = append(s.subs, sub)
	return sub
}

// Close shuts down the service.
func (s *serviceImpl) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.done)
	s.mu.Unlock()

	s.subsMu.Lock()
	for _, sub := range s.subs {
		sub.close()
	}
	s.subs = nil
	s.subsMu.Unlock()

	return nil
}

// emitStateChange notifies all subscribers of a state change.
// Must be called while holding mu. Acquires subsMu internally.
func (s *serviceImpl) emitStateChange(prev, curr State) {
	if prev == curr {
		return
	}
	e := StateChange{Previous: prev, Current: curr}
	s.subsMu.RLock()
	for _, sub := range s.subs {
		sub.sendState(e)
	}
	s.subsMu.RUnlock()
}

// Play starts playback of the current track in the queue.
func (s *serviceImpl) Play() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue.Tracks()) == 0 {
		return ErrEmptyQueue
	}

	track := s.queue.Current()
	if track == nil {
		return ErrNoCurrentTrack
	}

	prevState := s.playerStateToState(s.player.State())
	if err := s.player.Play(track.Path); err != nil {
		return err
	}
	currState := s.playerStateToState(s.player.State())
	s.emitStateChange(prevState, currState)
	return nil
}

// Pause pauses playback if currently playing.
func (s *serviceImpl) Pause() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.player.State() != player.Playing {
		return nil // no-op
	}

	prevState := s.playerStateToState(s.player.State())
	s.player.Pause()
	currState := s.playerStateToState(s.player.State())
	s.emitStateChange(prevState, currState)
	return nil
}

// Stop stops playback.
func (s *serviceImpl) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.player.State() == player.Stopped {
		return nil // no-op
	}

	prevState := s.playerStateToState(s.player.State())
	s.player.Stop()
	currState := s.playerStateToState(s.player.State())
	s.emitStateChange(prevState, currState)
	return nil
}

// Toggle toggles between play and pause states.
func (s *serviceImpl) Toggle() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prevState := s.playerStateToState(s.player.State())

	switch s.player.State() {
	case player.Playing:
		s.player.Pause()
	case player.Paused:
		s.player.Resume()
	case player.Stopped:
		// Play current track if available
		if len(s.queue.Tracks()) == 0 {
			return ErrEmptyQueue
		}
		track := s.queue.Current()
		if track == nil {
			return ErrNoCurrentTrack
		}
		if err := s.player.Play(track.Path); err != nil {
			return err
		}
	}

	currState := s.playerStateToState(s.player.State())
	s.emitStateChange(prevState, currState)
	return nil
}

func (s *serviceImpl) Next() error { return nil }

func (s *serviceImpl) Previous() error { return nil }

func (s *serviceImpl) Seek(_ time.Duration) error { return nil }

func (s *serviceImpl) SeekTo(_ time.Duration) error { return nil }

func (s *serviceImpl) JumpTo(_ int) error { return nil }

func (s *serviceImpl) SetRepeatMode(_ RepeatMode) {}

func (s *serviceImpl) CycleRepeatMode() RepeatMode { return RepeatOff }

func (s *serviceImpl) SetShuffle(_ bool) {}

func (s *serviceImpl) ToggleShuffle() bool { return false }
