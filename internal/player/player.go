package player

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
)

type State int

const (
	Stopped State = iota
	Playing
	Paused
)

type Player struct {
	state      State
	ctrl       *beep.Ctrl
	streamer   beep.StreamSeekCloser
	format     beep.Format
	file       *os.File
	trackInfo  *TrackInfo
	done       chan struct{}
	onFinished func()
}

type TrackInfo struct {
	Path     string
	Title    string
	Artist   string
	Album    string
	Year     int
	Track    int
	Duration time.Duration
}

var speakerInitialized bool

func New() *Player {
	return &Player{
		state: Stopped,
		done:  make(chan struct{}),
	}
}

func (p *Player) Play(path string) error {
	p.Stop()

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".mp3" && ext != ".flac" {
		return fmt.Errorf("unsupported format: %s", ext)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(f)
	case ".flac":
		streamer, format, err = flac.Decode(f)
	}
	if err != nil {
		f.Close()
		return err
	}

	if !speakerInitialized {
		err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
		if err != nil {
			streamer.Close()
			f.Close()
			return err
		}
		speakerInitialized = true
	}

	p.file = f
	p.streamer = streamer
	p.format = format
	p.ctrl = &beep.Ctrl{Streamer: streamer, Paused: false}

	info, _ := ReadTrackInfo(path)
	if info != nil {
		info.Duration = format.SampleRate.D(streamer.Len())
	} else {
		info = &TrackInfo{
			Path:     path,
			Title:    filepath.Base(path),
			Duration: format.SampleRate.D(streamer.Len()),
		}
	}
	p.trackInfo = info

	p.state = Playing
	p.done = make(chan struct{})

	speaker.Play(beep.Seq(p.ctrl, beep.Callback(func() {
		close(p.done)
		if p.onFinished != nil {
			p.onFinished()
		}
	})))

	return nil
}

func (p *Player) Stop() {
	if p.state == Stopped {
		return
	}

	speaker.Clear()

	if p.streamer != nil {
		p.streamer.Close()
		p.streamer = nil
	}
	if p.file != nil {
		p.file.Close()
		p.file = nil
	}

	p.ctrl = nil
	p.trackInfo = nil
	p.state = Stopped
}

func (p *Player) Pause() {
	if p.state != Playing || p.ctrl == nil {
		return
	}
	speaker.Lock()
	p.ctrl.Paused = true
	speaker.Unlock()
	p.state = Paused
}

func (p *Player) Resume() {
	if p.state != Paused || p.ctrl == nil {
		return
	}
	speaker.Lock()
	p.ctrl.Paused = false
	speaker.Unlock()
	p.state = Playing
}

func (p *Player) Toggle() {
	switch p.state {
	case Playing:
		p.Pause()
	case Paused:
		p.Resume()
	}
}

func (p *Player) State() State { return p.state }

func (p *Player) TrackInfo() *TrackInfo { return p.trackInfo }

func (p *Player) Position() time.Duration {
	if p.streamer == nil {
		return 0
	}
	speaker.Lock()
	pos := p.format.SampleRate.D(p.streamer.Position())
	speaker.Unlock()
	return pos
}

func (p *Player) Duration() time.Duration {
	if p.trackInfo == nil {
		return 0
	}
	return p.trackInfo.Duration
}

func (p *Player) OnFinished(fn func()) {
	p.onFinished = fn
}
