package export

import (
	"fmt"
	"path/filepath"
	"strings"
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

	// Truncate to 200 chars for FAT32 safety
	if len(result) > 200 {
		result = result[:200]
	}

	return result
}
