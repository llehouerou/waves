package player

import (
	"os"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"

	"github.com/llehouerou/waves/internal/tags"
)

// State represents the player's playback state.
type State int

const (
	Stopped State = iota
	Playing
	Paused
)

const (
	extMP3  = ".mp3"
	extFLAC = ".flac"
	extOPUS = ".opus"
	extOGG  = ".ogg"
	extOGA  = ".oga"
	extM4A  = ".m4a"
	extMP4  = ".mp4"
)

// trackState bundles all resources for a single track.
type trackState struct {
	file      *os.File
	streamer  beep.StreamSeekCloser
	resampled beep.Streamer // Resampled to speaker rate (may equal streamer)
	format    beep.Format
	trackInfo *tags.FileInfo
}

// Close releases all resources for this track.
func (t *trackState) Close() {
	if t.streamer != nil {
		t.streamer.Close()
	}
	if t.file != nil {
		t.file.Close()
	}
}

// Player handles audio playback.
type Player struct {
	state  State
	ctrl   *beep.Ctrl
	volume *effects.Volume

	volumeLevel float64 // 0.0 to 1.0
	muted       bool

	// Dual track state for gapless playback
	current *trackState
	next    *trackState
	gapless *gaplessStreamer

	// Channels
	done       chan struct{}
	finishedCh chan struct{}
	onFinished func()
	seekChan   chan time.Duration

	// Pre-loading
	preloadAt   time.Duration // How early to pre-load (default 3s)
	preloadFn   func() string // Callback to get next track path
	monitorDone chan struct{} // Stops the monitor loop
}

var (
	speakerInitialized bool
	speakerSampleRate  beep.SampleRate
)

// New creates a new Player.
func New() *Player {
	p := &Player{
		state:       Stopped,
		volumeLevel: 1.0, // Full volume by default
		done:        make(chan struct{}),
		finishedCh:  make(chan struct{}, 1), // buffered to avoid blocking
		seekChan:    make(chan time.Duration, 1),
		preloadAt:   3 * time.Second,
	}
	go p.seekLoop()
	return p
}

// FinishedChan returns a channel that receives when a track finishes naturally.
func (p *Player) FinishedChan() <-chan struct{} {
	return p.finishedCh
}

// Done returns a channel that is closed when the current track ends (naturally or stopped).
func (p *Player) Done() <-chan struct{} {
	return p.done
}

// State returns the current playback state.
func (p *Player) State() State { return p.state }

// TrackInfo returns metadata about the currently playing track.
func (p *Player) TrackInfo() *tags.FileInfo {
	if p.current == nil {
		return nil
	}
	return p.current.trackInfo
}

// Duration returns the total duration of the current track.
func (p *Player) Duration() time.Duration {
	if p.current == nil || p.current.trackInfo == nil {
		return 0
	}
	return p.current.trackInfo.Duration
}

// OnFinished sets a callback to be called when a track finishes.
func (p *Player) OnFinished(fn func()) {
	p.onFinished = fn
}

// SetPreloadFunc sets the callback to get the next track path for pre-loading.
func (p *Player) SetPreloadFunc(fn func() string) {
	p.preloadFn = fn
}

// SetPreloadDuration sets how early to pre-load the next track.
func (p *Player) SetPreloadDuration(d time.Duration) {
	p.preloadAt = d
}

// clearNextTrack closes and clears the pre-loaded next track.
func (p *Player) clearNextTrack() {
	if p.next != nil {
		go p.next.Close()
		p.next = nil
	}
	if p.gapless != nil {
		p.gapless.ClearNext()
	}
}
