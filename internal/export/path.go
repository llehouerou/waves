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

// GenerateExportPath creates the relative path for an exported track. No
// folder or file name of it starts with a dot (unhide): none is hidden, and
// none is "." or "..", so that the path stays under the root it is joined to.
func GenerateExportPath(t TrackInfo, structure FolderStructure) string {
	artist := sanitizeFilename(t.Artist)
	album := sanitizeFilename(t.Album)
	title := sanitizeFilename(t.Title)

	trackNum := formatTrackNumber(t.TrackNumber, t.DiscNumber, t.TotalDiscs)
	// The file name starts with the track number, never with a dot.
	file := fmt.Sprintf("%s - %s%s", trackNum, title, t.Extension)

	switch structure {
	case FolderStructureFlat:
		// Artist - Album/01 - Track.ext
		return filepath.Join(unhide(fmt.Sprintf("%s - %s", artist, album)), file)

	case FolderStructureSingle:
		// Artist - Album - 01 - Track.ext
		return unhide(fmt.Sprintf("%s - %s - %s", artist, album, file))

	case FolderStructureHierarchical:
	}
	// Artist/Album/01 - Track.ext: the hierarchical layout, and a layout this
	// code does not know.
	return filepath.Join(unhide(artist), unhide(album), file)
}

// unhide replaces the leading dot of a folder or file name with "_", as an
// import does. It runs last, on the name as the cleaning and the cut leave it:
// the name keeps its length.
func unhide(name string) string {
	if strings.HasPrefix(name, ".") {
		return "_" + name[1:]
	}
	return name
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
