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
	"github.com/gopxl/beep/v2/speaker"

	"github.com/llehouerou/waves/internal/tags"
)

// openTrack opens and decodes an audio file, returning a trackState.
func (p *Player) openTrack(path string) (*trackState, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != extMP3 && ext != extFLAC && ext != extOPUS && ext != extOGG && ext != extOGA && ext != extM4A && ext != extMP4 {
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format
	var m4aCodec string

	switch ext {
	case extMP3:
		// go-mp3 v1.2.0+ handles LAME/Xing gapless info automatically
		streamer, format, err = decodeGoMP3(f)
	case extFLAC:
		if err := skipID3v2(f); err != nil {
			f.Close()
			return nil, err
		}
		streamer, format, err = flac.Decode(f)
	case extOPUS, extOGG, extOGA:
		streamer, format, err = decodeOgg(f)
	case extM4A, extMP4:
		streamer, format, m4aCodec, err = decodeM4A(f)
	}
	if err != nil {
		f.Close()
		return nil, err
	}

	// Initialize speaker on first track
	if !speakerInitialized {
		speakerSampleRate = format.SampleRate
		err = speaker.Init(speakerSampleRate, speakerSampleRate.N(time.Second/10))
		if err != nil {
			streamer.Close()
			f.Close()
			return nil, err
		}
		speakerInitialized = true
	}

	// Resample if needed
	var resampled beep.Streamer = streamer
	if format.SampleRate != speakerSampleRate {
		resampled = beep.Resample(4, format.SampleRate, speakerSampleRate, streamer)
	}

	// Build track info
	tagInfo, _ := tags.Read(path)
	info := &tags.FileInfo{}
	if tagInfo != nil {
		info.Tag = *tagInfo
	} else {
		info.Path = path
		info.Title = filepath.Base(path)
	}
	info.Duration = format.SampleRate.D(streamer.Len())
	info.SampleRate = int(format.SampleRate)
	info.BitDepth = format.Precision * 8
	switch ext {
	case extMP3:
		info.Format = "MP3"
	case extOPUS:
		info.Format = "OPUS"
	case extOGG, extOGA:
		if IsOpusCodec(path) {
			info.Format = "OPUS"
		} else {
			info.Format = "VORBIS"
		}
	case extM4A, extMP4:
		info.Format = m4aCodec
	default:
		info.Format = "FLAC"
	}

	return &trackState{
		file:      f,
		streamer:  streamer,
		resampled: resampled,
		format:    format,
		trackInfo: info,
	}, nil
}

// Play starts playback of the given audio file.
func (p *Player) Play(path string) error {
	p.Stop()

	// Small delay to let any pending Beep callback complete after speaker.Clear()
	time.Sleep(10 * time.Millisecond)

	// Drain any stale finish signal from previous track
	select {
	case <-p.finishedCh:
	default:
	}

	track, err := p.openTrack(path)
	if err != nil {
		return err
	}

	p.current = track
	p.clearNextTrack()

	p.gapless = &gaplessStreamer{
		current:  track.resampled,
		onSwitch: p.handleGaplessTransition,
	}

	p.ctrl = &beep.Ctrl{Streamer: p.gapless, Paused: false}
	p.volume = &effects.Volume{
		Streamer: p.ctrl,
		Base:     2,
		Volume:   p.levelToVolume(p.volumeLevel),
		Silent:   p.muted,
	}

	p.state = Playing
	p.done = make(chan struct{})

	if p.monitorDone != nil {
		close(p.monitorDone)
	}
	p.monitorDone = make(chan struct{})
	go p.monitorLoop()

	speaker.Play(beep.Seq(p.volume, beep.Callback(func() {
		close(p.done)
		select {
		case p.finishedCh <- struct{}{}:
		default:
		}
		if p.onFinished != nil {
			p.onFinished()
		}
	})))

	return nil
}

// monitorLoop periodically checks if pre-loading should start.
func (p *Player) monitorLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if p.shouldPreload() {
				p.preloadNext()
			}
		case <-p.monitorDone:
			return
		}
	}
}

// shouldPreload returns true if we should start pre-loading the next track.
func (p *Player) shouldPreload() bool {
	if p.current == nil || p.next != nil || p.preloadFn == nil {
		return false
	}
	if p.state != Playing {
		return false
	}
	remaining := p.Duration() - p.Position()
	return remaining <= p.preloadAt && remaining > 0
}

// preloadNext loads the next track in the background.
func (p *Player) preloadNext() {
	path := p.preloadFn()
	if path == "" {
		return
	}

	track, err := p.openTrack(path)
	if err != nil {
		return // Silent failure - fall back to non-gapless
	}

	speaker.Lock()
	p.next = track
	if p.gapless != nil {
		p.gapless.SetNext(track.resampled)
	}
	speaker.Unlock()
}

// handleGaplessTransition is called when the gapless streamer transitions.
func (p *Player) handleGaplessTransition() {
	old := p.current
	if old != nil {
		go old.Close()
	}

	p.current = p.next
	p.next = nil

	select {
	case p.finishedCh <- struct{}{}:
	default:
	}
}

// ClearPreload removes the pre-loaded next track.
func (p *Player) ClearPreload() {
	speaker.Lock()
	defer speaker.Unlock()
	p.clearNextTrack()
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
