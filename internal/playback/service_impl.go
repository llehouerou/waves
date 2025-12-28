// internal/playback/service_impl.go
package playback

import (
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
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

// Stub implementations for interface compliance (will be implemented in later tasks)

func (s *serviceImpl) Play() error { return nil }

func (s *serviceImpl) Pause() error { return nil }

func (s *serviceImpl) Stop() error { return nil }

func (s *serviceImpl) Toggle() error { return nil }

func (s *serviceImpl) Next() error { return nil }

func (s *serviceImpl) Previous() error { return nil }

func (s *serviceImpl) Seek(_ time.Duration) error { return nil }

func (s *serviceImpl) SeekTo(_ time.Duration) error { return nil }

func (s *serviceImpl) JumpTo(_ int) error { return nil }

func (s *serviceImpl) SetRepeatMode(_ RepeatMode) {}

func (s *serviceImpl) CycleRepeatMode() RepeatMode { return RepeatOff }

func (s *serviceImpl) SetShuffle(_ bool) {}

func (s *serviceImpl) ToggleShuffle() bool { return false }
