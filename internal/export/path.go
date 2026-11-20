package export

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// TrackInfo contains metadata needed for export path generation.
type TrackInfo struct {
	Artist      string
	Album       string
	Title       string
	TrackNumber int
	DiscNumber  int
	TotalDiscs  int
	Extension   string // e.g., ".flac", ".mp3"
}

// GenerateExportPath creates the relative path for an exported track.
func GenerateExportPath(t TrackInfo, structure FolderStructure) string {
	artist := sanitizeFilename(t.Artist)
	album := sanitizeFilename(t.Album)
	title := sanitizeFilename(t.Title)

	trackNum := formatTrackNumber(t.TrackNumber, t.DiscNumber, t.TotalDiscs)
	ext := t.Extension

	switch structure {
	case FolderStructureFlat:
		// Artist - Album/01 - Track.ext
		folder := fmt.Sprintf("%s - %s", artist, album)
		file := fmt.Sprintf("%s - %s%s", trackNum, title, ext)
		return filepath.Join(folder, file)

	case FolderStructureHierarchical:
		// Artist/Album/01 - Track.ext
		file := fmt.Sprintf("%s - %s%s", trackNum, title, ext)
		return filepath.Join(artist, album, file)

	case FolderStructureSingle:
		// Artist - Album - 01 - Track.ext
		return fmt.Sprintf("%s - %s - %s - %s%s", artist, album, trackNum, title, ext)

	default:
		return filepath.Join(artist, album, fmt.Sprintf("%s - %s%s", trackNum, title, ext))
	}
}

// formatTrackNumber formats track number, including disc for multi-disc albums.
func formatTrackNumber(track, disc, totalDiscs int) string {
	if totalDiscs > 1 && disc > 0 {
		return fmt.Sprintf("%d-%02d", disc, track)
	}
	return fmt.Sprintf("%02d", track)
}

// maxNameBytes is the longest artist, album or title an export writes, in bytes.
const maxNameBytes = 200

// sanitizeFilename replaces illegal characters for FAT32 compatibility.
func sanitizeFilename(s string) string {
	// Characters not allowed in FAT32: / \ : * ? " < > |
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	result := replacer.Replace(s)

	// Truncate to 200 bytes for FAT32 safety, at a character boundary so that
	// the name stays valid UTF-8. A character starts at most UTFMax-1 bytes
	// back: a name that is not valid UTF-8 is cut there, never emptied.
	if len(result) > maxNameBytes {
		cut := maxNameBytes
		for cut > maxNameBytes-utf8.UTFMax+1 && !utf8.RuneStart(result[cut]) {
			cut--
		}
		result = result[:cut]
	}

	return result
}
