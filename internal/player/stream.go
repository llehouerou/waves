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
)

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

	ext := strings.ToLower(filepath.Ext(path))
	if ext != extMP3 && ext != extFLAC && ext != extOPUS && ext != extOGG && ext != extM4A && ext != extMP4 {
		return fmt.Errorf("unsupported format: %s", ext)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}

	var streamer beep.StreamSeekCloser
	var format beep.Format
	var m4aCodec string // For M4A files: "AAC" or "ALAC"

	switch ext {
	case extMP3:
		streamer, format, err = decodeGoMP3(f)
	case extFLAC:
		// Skip ID3v2 tag if present (some taggers add it to FLAC files)
		if err := skipID3v2(f); err != nil {
			f.Close()
			return err
		}
		streamer, format, err = flac.Decode(f)
	case extOPUS, extOGG:
		streamer, format, err = decodeOpus(f)
	case extM4A, extMP4:
		streamer, format, m4aCodec, err = decodeM4A(f)
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
		switch ext {
		case extMP3:
			info.Format = "MP3"
		case extOPUS, extOGG:
			info.Format = "OPUS"
		case extM4A, extMP4:
			info.Format = m4aCodec
		default:
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
