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
	files       map[string]*tags.FileInfo // what SetFileInfo says the files hold
	playErr     error
	playCalls   []string
	seekCalls   []time.Duration
	finishedCh  chan uint64
	done        chan struct{}
	volumeLevel float64
	muted       bool
	starts      uint64
	blockPlay   chan struct{}
	preloadFn   func() string
	clears      int
}

// NewMock creates a new mock player for testing.
func NewMock() *Mock {
	return &Mock{
		state:       Stopped,
		volumeLevel: 1.0,
		finishedCh:  make(chan uint64, 1),
		done:        make(chan struct{}),
	}
}

func (m *Mock) Play(path string) error {
	// Opening a track is real file I/O in the player; blockPlay lets a test model
	// a slow or unreachable file. The call is recorded before blocking, so a test
	// can tell that opening has started.
	m.mu.Lock()
	m.playCalls = append(m.playCalls, path)
	block := m.blockPlay
	m.mu.Unlock()
	if block != nil {
		<-block
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.playErr != nil {
		// Like the real player: it stops what was playing before it opens the
		// file, so a failed open leaves it stopped.
		m.state = Stopped
		return m.playErr
	}
	m.state = Playing
	// Like the real player: the track being played is now this one.
	m.trackInfo = m.fileInfo(path)
	m.starts++
	return nil
}

// fileInfo is what the file at path holds, as the player reads it when it
// opens it: its path alone unless SetFileInfo said more. Must be called with
// mu held.
func (m *Mock) fileInfo(path string) *tags.FileInfo {
	if info, ok := m.files[path]; ok {
		return info
	}
	return &tags.FileInfo{Tag: tags.Tag{Path: path}}
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

func (m *Mock) FinishedChan() <-chan uint64 {
	return m.finishedCh
}

func (m *Mock) Done() <-chan struct{} {
	return m.done
}

func (m *Mock) SetPreloadFunc(fn func() string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preloadFn = fn
}

func (m *Mock) SetPreloadDuration(_ time.Duration) {}

func (m *Mock) ClearPreload() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clears++
}

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

// SetFileInfo says what the file at info.Path holds: the mock reads it each
// time it opens that file, as the real player does, before the track starts.
func (m *Mock) SetFileInfo(info *tags.FileInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.files == nil {
		m.files = make(map[string]*tags.FileInfo)
	}
	m.files[info.Path] = info
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

// TrackStarts counts playback starts, like the real player.
func (m *Mock) TrackStarts() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.starts
}

// PreloadFunc returns the callback the player would ask for the next track.
func (m *Mock) PreloadFunc() func() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.preloadFn
}

// PreloadClears counts ClearPreload calls.
func (m *Mock) PreloadClears() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clears
}

// SimulateGaplessSwitch simulates the player moving to the next track by
// itself, as gapless playback does: the track playing finishes into it.
func (m *Mock) SimulateGaplessSwitch(path string) {
	m.SimulateGaplessStart(path)()
}

// SimulateGaplessStart makes the player move to the next track by itself, as
// SimulateGaplessSwitch does, but holds the finish of the track it left: the
// finish is signalled when the returned func is called. It lets a test act in
// between, as a command that takes the service lock before the service handles
// the finish does.
func (m *Mock) SimulateGaplessStart(path string) (finish func()) {
	m.mu.Lock()
	ended := m.starts
	m.trackInfo = m.fileInfo(path)
	m.state = Playing
	m.starts++
	m.mu.Unlock()
	return func() { m.finish(ended) }
}

// BlockPlay makes Play block until the returned func is called, modelling a
// track that is slow to open.
func (m *Mock) BlockPlay() (release func()) {
	ch := make(chan struct{})
	m.mu.Lock()
	m.blockPlay = ch
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		m.blockPlay = nil
		m.mu.Unlock()
		close(ch)
	}
}

// SimulateFinished simulates the track last started finishing.
func (m *Mock) SimulateFinished() {
	m.finish(m.TrackStarts())
}

// SimulateFinishedOf simulates the finish of the track the player started as
// start (TrackStarts once it started it), whatever it is on now: a finish the
// service must take as stale.
func (m *Mock) SimulateFinishedOf(start uint64) {
	m.finish(start)
}

func (m *Mock) finish(start uint64) {
	select {
	case m.finishedCh <- start:
	default:
	}
}

// Verify Mock implements Interface at compile time.
var _ Interface = (*Mock)(nil)
