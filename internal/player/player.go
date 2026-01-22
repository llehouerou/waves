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
	extM4A  = ".m4a"
	extMP4  = ".mp4"
)

// Player handles audio playback.
type Player struct {
	state      State
	ctrl       *beep.Ctrl
	volume     *effects.Volume
	streamer   beep.StreamSeekCloser
	format     beep.Format
	file       *os.File
	trackInfo  *tags.FileInfo
	done       chan struct{}
	finishedCh chan struct{}
	onFinished func()

	// Seek state - only latest seek is processed, others are dropped
	seekChan chan time.Duration
}

var (
	speakerInitialized bool
	speakerSampleRate  beep.SampleRate
)

// New creates a new Player.
func New() *Player {
	p := &Player{
		state:      Stopped,
		done:       make(chan struct{}),
		finishedCh: make(chan struct{}, 1), // buffered to avoid blocking
		seekChan:   make(chan time.Duration, 1),
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
func (p *Player) TrackInfo() *tags.FileInfo { return p.trackInfo }

// Duration returns the total duration of the current track.
func (p *Player) Duration() time.Duration {
	if p.trackInfo == nil {
		return 0
	}
	return p.trackInfo.Duration
}

// OnFinished sets a callback to be called when a track finishes.
func (p *Player) OnFinished(fn func()) {
	p.onFinished = fn
}
