package tags

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

// Read reads tag metadata from a music file.
// It returns only tag metadata, not audio stream properties.
func Read(path string) (*Tag, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ExtMP3:
			// dhowden/tag has issues with some UTF-16 encoded ID3 tags
			return readMP3WithID3v2Fallback(path)
		case ExtM4A, ExtMP4:
			// dhowden/tag can't parse some M4A files (e.g., ffmpeg-created)
			return readM4AWithTaglib(path)
		case ExtFLAC:
			// dhowden/tag can fail on some FLAC files
			return readFLACWithTaglib(path)
		case ExtOPUS, ExtOGG, ExtOGA:
			// dhowden/tag can fail on some Ogg files
			return readOggWithTaglib(path)
		}
		return nil, err
	}

	title := m.Title()
	if title == "" {
		title = filepath.Base(path)
	}

	track, totalTracks := m.Track()
	disc, totalDiscs := m.Disc()

	albumArtist := m.AlbumArtist()
	if albumArtist == "" {
		albumArtist = m.Artist()
	}

	t := &Tag{
		Path:        path,
		Title:       title,
		Artist:      m.Artist(),
		AlbumArtist: albumArtist,
		Album:       m.Album(),
		Date:        yearToDate(m.Year()),
		TrackNumber: track,
		TotalTracks: totalTracks,
		DiscNumber:  disc,
		TotalDiscs:  totalDiscs,
		Genre:       m.Genre(),
	}

	// Read extended tags based on file format
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ExtMP3:
		readMP3ExtendedTags(path, t)
	case ExtFLAC:
		readFLACExtendedTags(path, t)
	case ExtOPUS, ExtOGG, ExtOGA:
		readOggExtendedTags(path, t)
	case ExtM4A, ExtMP4:
		readM4AExtendedTags(path, t)
	}

	return t, nil
}

// ReadWithAudio reads both tag metadata and audio stream properties.
func ReadWithAudio(path string) (*FileInfo, error) {
	t, err := Read(path)
	if err != nil {
		// If tag reading fails, create basic info from filename
		t = &Tag{
			Path:  path,
			Title: filepath.Base(path),
		}
	}

	audio, err := ReadAudioInfo(path)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Tag:       *t,
		AudioInfo: *audio,
	}, nil
}

// yearToDate converts a year integer to a date string.
// Returns empty string for year 0.
func yearToDate(year int) string {
	if year == 0 {
		return ""
	}
	return strconv.Itoa(year)
}
