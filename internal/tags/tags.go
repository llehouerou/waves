// Package tags provides unified tag reading and writing for music files.
// It consolidates metadata handling for MP3, FLAC, Opus, and M4A formats.
package tags

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// File extensions supported by the tags package.
const (
	ExtMP3  = ".mp3"
	ExtFLAC = ".flac"
	ExtOPUS = ".opus"
	ExtOGG  = ".ogg"
	ExtOGA  = ".oga" // Ogg Audio container (Vorbis/Opus)
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
	Media         string // Format of the medium the track is on (CD, Vinyl, Digital Media, etc.), as Picard writes it
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

	// Artwork (write-only, not populated during read). On a write, empty
	// means no new cover: the file keeps the pictures it embeds, in every
	// format.
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

// applyFallbacks fills the fields a file may leave empty, once every reader
// has set the values: the title falls back on the file name, the album artist
// on the artist.
func (t *Tag) applyFallbacks() {
	if t.Title == "" {
		t.Title = filepath.Base(t.Path)
	}
	if t.AlbumArtist == "" {
		t.AlbumArtist = t.Artist
	}
}

// Sanitize cleans every text field by removing control characters and
// invalid UTF-8 bytes, and turning non-breaking spaces into spaces. This
// prevents broken display from bad metadata, and IDs that MusicBrainz does
// not know.
func (t *Tag) Sanitize() {
	for _, field := range []*string{
		&t.Title, &t.Artist, &t.AlbumArtist, &t.Album, &t.Genre,
		&t.Date, &t.OriginalDate, &t.ArtistSortName,
		&t.Label, &t.CatalogNumber, &t.Barcode, &t.Media,
		&t.ReleaseStatus, &t.ReleaseType, &t.Script, &t.Country, &t.ISRC,
		&t.MBArtistID, &t.MBReleaseID, &t.MBReleaseGroupID, &t.MBRecordingID, &t.MBTrackID,
	} {
		*field = sanitizeString(*field)
	}
}

// sanitizeString removes control characters and invalid UTF-8 bytes from a string.
func sanitizeString(s string) string {
	clean := false
	for i := range len(s) {
		b := s[i]
		if b < 0x20 || (b >= 0x80 && b <= 0x9f) {
			clean = true
			break
		}
		if b == 0xc2 && i+1 < len(s) && s[i+1] == 0xa0 {
			clean = true
			break
		}
	}
	if !clean {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			i++
			continue
		}
		if unicode.IsControl(r) {
			i += size
			continue
		}
		if r == '\u00a0' {
			b.WriteByte(' ')
			i += size
			continue
		}
		b.WriteString(s[i : i+size])
		i += size
	}
	return strings.TrimSpace(b.String())
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
	return ext == ExtMP3 || ext == ExtFLAC || ext == ExtOPUS || ext == ExtOGG || ext == ExtOGA || ext == ExtM4A || ext == ExtMP4
}
