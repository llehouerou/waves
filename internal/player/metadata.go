package player

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
)

func ReadTrackInfo(path string) (*TrackInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil, err
	}

	title := m.Title()
	if title == "" {
		title = filepath.Base(path)
	}

	track, _ := m.Track()

	albumArtist := m.AlbumArtist()
	if albumArtist == "" {
		albumArtist = m.Artist()
	}

	return &TrackInfo{
		Path:        path,
		Title:       title,
		Artist:      m.Artist(),
		AlbumArtist: albumArtist,
		Album:       m.Album(),
		Year:        m.Year(),
		Track:       track,
		Genre:       m.Genre(),
	}, nil
}

// ExtractFullMetadata reads both tag metadata and audio duration.
// It decodes the audio file to determine duration.
func ExtractFullMetadata(path string) (*TrackInfo, error) {
	// First get tag metadata
	info, err := ReadTrackInfo(path)
	if err != nil {
		// If tag reading fails, create basic info from filename
		info = &TrackInfo{
			Path:  path,
			Title: filepath.Base(path),
		}
	}

	// Now decode audio to get duration
	duration, err := getAudioDuration(path)
	if err != nil {
		return nil, err
	}
	info.Duration = duration

	return info, nil
}

func getAudioDuration(path string) (time.Duration, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != extMP3 && ext != extFLAC {
		return 0, fmt.Errorf("unsupported format: %s", ext)
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch ext {
	case extMP3:
		streamer, format, err = mp3.Decode(f)
	case extFLAC:
		if err := skipID3v2(f); err != nil {
			return 0, err
		}
		streamer, format, err = flac.Decode(f)
	}
	if err != nil {
		return 0, err
	}
	defer streamer.Close()

	return format.SampleRate.D(streamer.Len()), nil
}

func IsMusicFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == extMP3 || ext == extFLAC
}

// ExtractCoverArt reads embedded cover art from an audio file.
// Returns the image data and MIME type, or nil if no art is embedded.
func ExtractCoverArt(path string) (data []byte, mimeType string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil, "", err
	}

	pic := m.Picture()
	if pic == nil {
		return nil, "", nil
	}

	return pic.Data, pic.MIMEType, nil
}
