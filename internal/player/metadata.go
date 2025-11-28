package player

import (
	"os"
	"path/filepath"

	"github.com/dhowden/tag"
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

	return &TrackInfo{
		Path:   path,
		Title:  title,
		Artist: m.Artist(),
		Album:  m.Album(),
	}, nil
}

func IsMusicFile(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".mp3", ".flac", ".MP3", ".FLAC":
		return true
	}
	return false
}
