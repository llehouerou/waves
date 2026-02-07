// internal/player/mock.go
package player

import (
	"sync"
	"time"

	"github.com/llehouerou/waves/internal/tags"
)

// Mock is a test double for Player.
type Mock struct {
	mu          sync.Mutex
	state       State
	position    time.Duration
	duration    time.Duration
	trackInfo   *tags.FileInfo
	playErr     error
	playCalls   []string
	seekCalls   []time.Duration
	finishedCh  chan struct{}
	done        chan struct{}
	volumeLevel float64
	muted       bool
}

// NewMock creates a new mock player for testing.
func NewMock() *Mock {
	return &Mock{
		state:       Stopped,
		volumeLevel: 1.0,
		finishedCh:  make(chan struct{}, 1),
		done:        make(chan struct{}),
	}
}

func (m *Mock) Play(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.playCalls = append(m.playCalls, path)
	if m.playErr != nil {
		return m.playErr
	}
	m.state = Playing
	return nil
}

func (m *Mock) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = Stopped
}

func (m *Mock) Pause() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Playing {
		m.state = Paused
	}
}

func (m *Mock) Resume() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Paused {
		m.state = Playing
	}
}

func (m *Mock) Toggle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch m.state {
	case Playing:
		m.state = Paused
	case Paused:
		m.state = Playing
	case Stopped:
		// Nothing to toggle when stopped
	}
}

func (m *Mock) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Mock) TrackInfo() *tags.FileInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.trackInfo
}

func (m *Mock) Position() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.position
}

func (m *Mock) Duration() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.duration
}

func (m *Mock) Seek(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seekCalls = append(m.seekCalls, d)
}

func (m *Mock) OnFinished(_ func()) {}

func (m *Mock) FinishedChan() <-chan struct{} {
	return m.finishedCh
}

func (m *Mock) Done() <-chan struct{} {
	return m.done
}

func (m *Mock) SetPreloadFunc(_ func() string) {}

func (m *Mock) SetPreloadDuration(_ time.Duration) {}

func (m *Mock) ClearPreload() {}

func (m *Mock) SetVolume(level float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if level < 0 {
		level = 0
	}
	if level > 1 {
		level = 1
	}
	m.volumeLevel = level
}

func (m *Mock) Volume() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.volumeLevel
}

func (m *Mock) SetMuted(muted bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.muted = muted
}

func (m *Mock) Muted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.muted
}

// Test helpers

func (m *Mock) SetState(s State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = s
}

func (m *Mock) SetPlayError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.playErr = err
}

func (m *Mock) PlayCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.playCalls))
	copy(result, m.playCalls)
	return result
}

func (m *Mock) SeekCalls() []time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]time.Duration, len(m.seekCalls))
	copy(result, m.seekCalls)
	return result
}

func (m *Mock) SetTrackInfo(info *tags.FileInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trackInfo = info
}

func (m *Mock) SetDuration(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.duration = d
}

func (m *Mock) SetPosition(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.position = d
}

// SimulateFinished simulates a track finishing.
func (m *Mock) SimulateFinished() {
	select {
	case m.finishedCh <- struct{}{}:
	default:
	}
}

// Verify Mock implements Interface at compile time.
var _ Interface = (*Mock)(nil)
