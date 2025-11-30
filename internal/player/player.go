package player

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
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

const (
	extMP3  = ".mp3"
	extFLAC = ".flac"
)

type Player struct {
	state      State
	ctrl       *beep.Ctrl
	volume     *effects.Volume
	streamer   beep.StreamSeekCloser
	format     beep.Format
	file       *os.File
	trackInfo  *TrackInfo
	done       chan struct{}
	onFinished func()
}

type TrackInfo struct {
	Path        string
	Title       string
	Artist      string
	AlbumArtist string
	Album       string
	Year        int
	Track       int
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

func New() *Player {
	return &Player{
		state: Stopped,
		done:  make(chan struct{}),
	}
}

func (p *Player) Play(path string) error {
	p.Stop()

	ext := strings.ToLower(filepath.Ext(path))
	if ext != extMP3 && ext != extFLAC {
		return fmt.Errorf("unsupported format: %s", ext)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch ext {
	case extMP3:
		streamer, format, err = mp3.Decode(f)
	case extFLAC:
		// Skip ID3v2 tag if present (some taggers add it to FLAC files)
		if err := skipID3v2(f); err != nil {
			f.Close()
			return err
		}
		streamer, format, err = flac.Decode(f)
	}
	if err != nil {
		f.Close()
		return err
	}

	if !speakerInitialized {
		speakerSampleRate = format.SampleRate
		err = speaker.Init(speakerSampleRate, speakerSampleRate.N(time.Second/10))
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

	// Resample if the track's sample rate differs from the speaker's
	var playStreamer beep.Streamer = streamer
	if format.SampleRate != speakerSampleRate {
		playStreamer = beep.Resample(4, format.SampleRate, speakerSampleRate, streamer)
	}
	p.ctrl = &beep.Ctrl{Streamer: playStreamer, Paused: false}
	p.volume = &effects.Volume{Streamer: p.ctrl, Base: 2, Volume: 0, Silent: false}

	info, _ := ReadTrackInfo(path)
	if info != nil {
		info.Duration = format.SampleRate.D(streamer.Len())
		info.SampleRate = int(format.SampleRate)
		info.BitDepth = format.Precision * 8
		if ext == extMP3 {
			info.Format = "MP3"
		} else {
			info.Format = "FLAC"
		}
	} else {
		info = &TrackInfo{
			Path:       path,
			Title:      filepath.Base(path),
			Duration:   format.SampleRate.D(streamer.Len()),
			SampleRate: int(format.SampleRate),
			BitDepth:   format.Precision * 8,
			Format:     strings.ToUpper(strings.TrimPrefix(ext, ".")),
		}
	}
	p.trackInfo = info

	p.state = Playing
	p.done = make(chan struct{})

	speaker.Play(beep.Seq(p.volume, beep.Callback(func() {
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
	case Stopped:
		// Nothing to toggle when stopped
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

// Seek moves the playback position by the given delta.
// If seeking past the end, the player stops.
func (p *Player) Seek(delta time.Duration) {
	if p.streamer == nil || p.state == Stopped || p.volume == nil {
		return
	}

	speaker.Lock()
	currentPos := p.streamer.Position()
	newPos := currentPos + p.format.SampleRate.N(delta)
	maxPos := p.streamer.Len()

	// Stop if seeking past the end
	if newPos >= maxPos {
		speaker.Unlock()
		p.Stop()
		return
	}

	// Clamp to valid range
	newPos = max(newPos, 0)

	// Mute, seek, then unmute to avoid audio artifacts
	p.volume.Silent = true
	_ = p.streamer.Seek(newPos)
	speaker.Unlock()

	// Brief pause to let buffer clear before unmuting
	time.Sleep(100 * time.Millisecond)

	speaker.Lock()
	p.volume.Silent = false
	speaker.Unlock()
}

func (p *Player) OnFinished(fn func()) {
	p.onFinished = fn
}

// skipID3v2 skips an ID3v2 tag if present at the beginning of the file.
// Some FLAC files have ID3v2 tags prepended, which the FLAC decoder doesn't handle.
func skipID3v2(r io.ReadSeeker) error {
	// Read the first 10 bytes to check for ID3v2 header
	header := make([]byte, 10)
	n, err := r.Read(header)
	if err != nil {
		return err
	}
	if n < 10 {
		// File too small, seek back to start
		_, err = r.Seek(0, io.SeekStart)
		return err
	}

	// Check for "ID3" magic
	if string(header[0:3]) != "ID3" {
		// No ID3v2 tag, seek back to start
		_, err = r.Seek(0, io.SeekStart)
		return err
	}

	// ID3v2 size is stored as a syncsafe integer in bytes 6-9
	// Each byte only uses 7 bits (bit 7 is always 0)
	size := int64(header[6])<<21 | int64(header[7])<<14 | int64(header[8])<<7 | int64(header[9])

	// Total skip = 10 byte header + size
	_, err = r.Seek(10+size, io.SeekStart)
	return err
}
