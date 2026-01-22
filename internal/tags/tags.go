// Package tags provides unified tag reading and writing for music files.
// It consolidates metadata handling for MP3, FLAC, Opus, and M4A formats.
package tags

import (
	"strconv"
	"strings"
	"time"
)

// File extensions supported by the tags package.
const (
	ExtMP3  = ".mp3"
	ExtFLAC = ".flac"
	ExtOPUS = ".opus"
	ExtOGG  = ".ogg"
	ExtM4A  = ".m4a"
	ExtMP4  = ".mp4"
)

// id3Magic is the magic bytes for ID3v2 header detection.
const id3Magic = "ID3"

// Tag contains all music file tag metadata (read/write).
// This is the unified struct that replaces both player.TrackInfo and importer.TagData.
type Tag struct {
	Path        string
	Title       string
	Artist      string
	AlbumArtist string
	Album       string
	Genre       string

	// Track/disc numbering (unified naming)
	TrackNumber int
	TotalTracks int
	DiscNumber  int
	TotalDiscs  int

	// Date tags
	Date         string // Release date (YYYY-MM-DD or YYYY)
	OriginalDate string // Original release date

	// Artist info
	ArtistSortName string

	// Release info
	Label         string
	CatalogNumber string
	Barcode       string
	Media         string // Format (CD, Vinyl, Digital, etc.)
	ReleaseStatus string // Official, Promotional, Bootleg
	ReleaseType   string // Album, Single, EP, etc.
	Script        string // Latn, Cyrl, etc.
	Country       string

	// Recording info
	ISRC string // International Standard Recording Code

	// MusicBrainz IDs
	MBArtistID       string
	MBReleaseID      string
	MBReleaseGroupID string
	MBRecordingID    string
	MBTrackID        string

	// Artwork (write-only, not populated during read)
	CoverArt []byte
}

// Year derives the year from the Date field.
// Returns 0 if Date is empty or cannot be parsed.
func (t *Tag) Year() int {
	if t.Date == "" {
		return 0
	}
	// Date may be YYYY-MM-DD or just YYYY
	year := t.Date
	if len(year) > 4 {
		year = year[:4]
	}
	y, _ := strconv.Atoi(year)
	return y
}

// OriginalYear derives the year from the OriginalDate field.
// Returns empty string if OriginalDate is empty.
func (t *Tag) OriginalYear() string {
	if t.OriginalDate == "" {
		return ""
	}
	if len(t.OriginalDate) >= 4 {
		return t.OriginalDate[:4]
	}
	return t.OriginalDate
}

// AudioInfo contains audio stream properties (not tags).
type AudioInfo struct {
	Duration   time.Duration
	Format     string // MP3, FLAC, OPUS, M4A
	SampleRate int
	BitDepth   int
}

// FileInfo combines Tag and AudioInfo for a complete file description.
type FileInfo struct {
	Tag
	AudioInfo
}

// IsMusicFile returns true if the path has a supported music file extension.
func IsMusicFile(path string) bool {
	ext := strings.ToLower(path)
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx:]
	} else {
		return false
	}
	return ext == ExtMP3 || ext == ExtFLAC || ext == ExtOPUS || ext == ExtOGG || ext == ExtM4A || ext == ExtMP4
}

// taglibTags wraps a taglib result map with helper methods.
// This reduces duplication across format-specific readers.
type taglibTags map[string][]string

// get returns the first value for any of the given keys, or empty string if not found.
func (t taglibTags) get(keys ...string) string {
	for _, key := range keys {
		if values, ok := t[key]; ok && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// getInt returns the first value as an integer, or 0 if not found or invalid.
func (t taglibTags) getInt(key string) int {
	if values, ok := t[key]; ok && len(values) > 0 {
		if n, err := strconv.Atoi(values[0]); err == nil {
			return n
		}
	}
	return 0
}

// parseNumberPair parses a track/disc number that may be "N" or "N/M" format.
func (t taglibTags) parseNumberPair(key string) (num, total int) {
	s := t.get(key)
	if s == "" {
		return 0, 0
	}
	if idx := strings.Index(s, "/"); idx > 0 {
		num, _ = strconv.Atoi(s[:idx])
		total, _ = strconv.Atoi(s[idx+1:])
		return num, total
	}
	num, _ = strconv.Atoi(s)
	return num, 0
}
