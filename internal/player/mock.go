// internal/player/mock.go
package player

import "time"

// Mock is a test double for Player.
type Mock struct {
	state      State
	position   time.Duration
	duration   time.Duration
	trackInfo  *TrackInfo
	playErr    error
	playCalls  []string
	seekCalls  []time.Duration
	finishedCh chan struct{}
	done       chan struct{}
}

// NewMock creates a new mock player for testing.
func NewMock() *Mock {
	return &Mock{
		state:      Stopped,
		finishedCh: make(chan struct{}, 1),
		done:       make(chan struct{}),
	}
}

func (m *Mock) Play(path string) error {
	m.playCalls = append(m.playCalls, path)
	if m.playErr != nil {
		return m.playErr
	}
	m.state = Playing
	return nil
}

func (m *Mock) Stop() { m.state = Stopped }

func (m *Mock) Pause() {
	if m.state == Playing {
		m.state = Paused
	}
}

func (m *Mock) Resume() {
	if m.state == Paused {
		m.state = Playing
	}
}

func (m *Mock) Toggle() {
	switch m.state {
	case Playing:
		m.Pause()
	case Paused:
		m.Resume()
	case Stopped:
		// Nothing to toggle when stopped
	}
}

func (m *Mock) State() State { return m.state }

func (m *Mock) TrackInfo() *TrackInfo { return m.trackInfo }

func (m *Mock) Position() time.Duration { return m.position }

func (m *Mock) Duration() time.Duration { return m.duration }

func (m *Mock) Seek(d time.Duration) {
	m.seekCalls = append(m.seekCalls, d)
}

func (m *Mock) OnFinished(_ func()) {}

func (m *Mock) FinishedChan() <-chan struct{} {
	return m.finishedCh
}

func (m *Mock) Done() <-chan struct{} {
	return m.done
}

// Test helpers

func (m *Mock) SetState(s State) { m.state = s }

func (m *Mock) SetPlayError(err error) { m.playErr = err }

func (m *Mock) PlayCalls() []string { return m.playCalls }

func (m *Mock) SeekCalls() []time.Duration { return m.seekCalls }

func (m *Mock) SetTrackInfo(info *TrackInfo) { m.trackInfo = info }

func (m *Mock) SetDuration(d time.Duration) { m.duration = d }

func (m *Mock) SetPosition(d time.Duration) { m.position = d }

// SimulateFinished simulates a track finishing.
func (m *Mock) SimulateFinished() {
	select {
	case m.finishedCh <- struct{}{}:
	default:
	}
}

// Verify Mock implements Interface at compile time.
var _ Interface = (*Mock)(nil)
