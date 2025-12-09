package player

import (
	"os"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
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
)

// Player handles audio playback.
type Player struct {
	state      State
	ctrl       *beep.Ctrl
	volume     *effects.Volume
	streamer   beep.StreamSeekCloser
	format     beep.Format
	file       *os.File
	trackInfo  *TrackInfo
	done       chan struct{}
	finishedCh chan struct{}
	onFinished func()

	// Seek state - only latest seek is processed, others are dropped
	seekChan chan time.Duration
}

// TrackInfo contains metadata about the currently playing track.
type TrackInfo struct {
	Path        string
	Title       string
	Artist      string
	AlbumArtist string
	Album       string
	Year        int
	Track       int
	TotalTracks int
	Disc        int
	TotalDiscs  int
	Genre       string
	Duration    time.Duration
	Format      string // "MP3" or "FLAC"
	SampleRate  int    // e.g., 44100
	BitDepth    int    // e.g., 16, 24
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

// FinishedChan returns a channel that receives when a track finishes.
func (p *Player) FinishedChan() <-chan struct{} {
	return p.finishedCh
}

// State returns the current playback state.
func (p *Player) State() State { return p.state }

// TrackInfo returns metadata about the currently playing track.
func (p *Player) TrackInfo() *TrackInfo { return p.trackInfo }

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
