package rename

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Constants used in path construction
const (
	nonAlbum       = "[singles]"
	unknownArtist  = "unknown artist"
	unknownAlbum   = "unknown album"
	unknownTitle   = "unknown title"
	variousArtists = "Various Artists"
)

// Config holds the rename configuration.
type Config struct {
	Folder   string // Template for folder path
	Filename string // Template for filename (without extension)

	// Smart features
	ReissueNotation   bool // [YYYY reissue] suffix
	VABrackets        bool // [Various Artists] folder
	SinglesHandling   bool // [singles] folder, no album in filename
	ReleaseTypeNotes  bool // (soundtrack), (live), etc.
	AndToAmpersand    bool // "and" → "&"
	RemoveFeat        bool // Strip "feat." from titles
	EllipsisNormalize bool // "..." → "…"
}

// Default templates matching current hardcoded behavior
const (
	DefaultFolderTemplate   = "{albumartist}/{year} • {album}"
	DefaultFilenameTemplate = "{artist} • {album} • {tracknumber} · {title}"
)

// DefaultConfig returns a Config that produces output identical to the
// current hardcoded behavior.
func DefaultConfig() Config {
	return Config{
		Folder:            DefaultFolderTemplate,
		Filename:          DefaultFilenameTemplate,
		ReissueNotation:   true,
		VABrackets:        true,
		SinglesHandling:   true,
		ReleaseTypeNotes:  true,
		AndToAmpersand:    true,
		RemoveFeat:        true,
		EllipsisNormalize: true,
	}
}

// TrackMetadata contains all the metadata needed to generate a file path
type TrackMetadata struct {
	Artist               string
	AlbumArtist          string
	Album                string
	Title                string
	TrackNumber          int
	DiscNumber           int
	TotalDiscs           int
	Date                 string // Release date (YYYY or YYYY-MM-DD)
	OriginalDate         string // Original release date
	ReleaseType          string // album, single, ep, broadcast
	SecondaryReleaseType string // live, compilation, soundtrack, etc.
}

var (
	reAnd   = regexp.MustCompile(`(?i)\sand\s`)
	re3Dots = regexp.MustCompile(`\.{3}`)
	// reFeat matches a credit word and everything after it. A credit word is
	// "feat" or "ft" (lowercase, an optional dot after it) standing as a word
	// of its own: the character before it is whitespace or an opening bracket,
	// the character after it is not a letter or a digit, or it ends the name.
	// 1. " feat. Artist" or " ft. Artist" (unbracketed): matched from the
	//    whitespace before the credit word.
	// 2. " (feat. Artist)" or " [Remix ft. Artist]" (bracketed): matched from
	//    the whitespace before the opening bracket.
	reFeat = regexp.MustCompile(`\s+(?:[\[\({](?:[^)\]}]*[\s\[\({])?)?(?:feat|ft)(?:[^\pL\pN].*)?$`)
	// reQuestionMarks matches ? and ¿
	reQuestionMarks = regexp.MustCompile(`[¿?]+`)
	// reQuoteMarks matches various quote marks (double, fancy double, fancy single)
	// U+0022 ("), U+201C ("), U+201D ("), U+2018 ('), U+2019 (')
	reQuoteMarks = regexp.MustCompile(`["\x{201c}\x{201d}\x{2018}\x{2019}]+`)
	// reIllegalFileChars matches characters not allowed in filenames, with surrounding whitespace
	// Includes: / \ > < * : _ |
	reIllegalFileChars = regexp.MustCompile(`\s*[/\\><*:_|]+\s*`)
	// reEndPeriod matches a period at the end of a string
	reEndPeriod = regexp.MustCompile(`\.$`)
	// reMultiSpace matches multiple whitespace characters
	reMultiSpace = regexp.MustCompile(`\s+`)
)

// removeQuestionMarks removes ? and ¿ characters
func removeQuestionMarks(s string) string {
	return reQuestionMarks.ReplaceAllString(s, "")
}

// replaceQuoteMarks replaces various quote marks with single quotes
func replaceQuoteMarks(s string) string {
	return reQuoteMarks.ReplaceAllString(s, "'")
}

// replaceIllegalFileChars replaces illegal filename characters with " - "
func replaceIllegalFileChars(s string) string {
	return reIllegalFileChars.ReplaceAllString(s, " - ")
}

// removeEndPeriod removes trailing period from a string
func removeEndPeriod(s string) string {
	return reEndPeriod.ReplaceAllString(s, "")
}

// normalizeSpaces trims and reduces multiple whitespace to single space
func normalizeSpaces(s string) string {
	s = reMultiSpace.ReplaceAllString(s, " ")
	// Trim leading/trailing spaces
	if s != "" && s[0] == ' ' {
		s = s[1:]
	}
	if s != "" && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}

// removeFeatPatterns removes a credit word ("feat" or "ft" as a word of its
// own, see reFeat) and everything after it; inside brackets, the whole bracket
// goes too. Any other name is returned unchanged.
func removeFeatPatterns(s string) string {
	return reFeat.ReplaceAllString(s, "")
}

// replace3DotsWithEllipsis replaces "..." with "…"
func replace3DotsWithEllipsis(s string) string {
	return re3Dots.ReplaceAllString(s, "…")
}

// replaceAndWithAmpersand replaces standalone "and" (case-insensitive) with "&"
func replaceAndWithAmpersand(s string) string {
	return reAnd.ReplaceAllString(s, " & ")
}

// Release types that go into album notes
var albumNoteTypes = []string{
	"soundtrack",
	"audiobook",
	"mixtape/street",
	"compilation",
	"ep",
}

// Release types that go into track notes
var trackNoteTypes = []string{
	"live",
	"broadcast",
	"spokenword",
	"interview",
	"remix",
	"dj-mix",
}

// cleanForFilename applies all transformations needed for safe filenames.
// The text options of the config (and, feat., ellipsis) are not among them:
// folderName and fileName apply them first, as configured.
func cleanForFilename(s string) string {
	s = removeQuestionMarks(s)
	s = replaceQuoteMarks(s)
	s = replaceIllegalFileChars(s)
	s = normalizeSpaces(s)
	return s
}

// cleanForFolder applies transformations for folder names (includes trailing period removal)
func cleanForFolder(s string) string {
	s = cleanForFilename(s)
	s = removeEndPeriod(s)
	return s
}

// nameOnDisk gives a cleaned folder or file name the form it takes on disk:
// a name the cleaning left empty (a tag "?", a title "¿?") is "_", and a
// leading dot becomes "_", as Picard writes it, so that no name of a generated
// path is missing or hidden. It runs last, on the name as the cleaning leaves
// it: the cleaning itself turns a "_" of a tag into " - ".
func nameOnDisk(name string) string {
	if name == "" {
		return "_"
	}
	if strings.HasPrefix(name, ".") {
		return "_" + name[1:]
	}
	return name
}

// folderName is the name of a folder of a generated path: s transformed as cfg
// says, cleaned, then given its form on disk. The text options run on the whole
// name, as well as on each placeholder value: "{year} • {album}" with an album
// "And Then There Were Three" gives "1978 • & Then There Were Three" when they
// are on.
func folderName(s string, cfg Config) string {
	return nameOnDisk(cleanForFolder(applyTextTransforms(s, cfg)))
}

// fileName is the name of the file of a generated path: s transformed as cfg
// says, cleaned, then given its form on disk, as folderName: the file of a
// template that gives no text is "_" too.
func fileName(s string, cfg Config) string {
	return nameOnDisk(cleanForFilename(applyTextTransforms(s, cfg)))
}

// getYear extracts the year (first 4 chars) from a date string
func getYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return date
}

// GeneratePath generates a file path from track metadata using default config.
// Output format: AlbumArtist/Year • Album/Artist • Album • DiscNum.TrackNum · Title
func GeneratePath(m TrackMetadata) string {
	return GeneratePathWithConfig(m, DefaultConfig())
}

// GeneratePathWithConfig generates a file path using the provided config.
func GeneratePathWithConfig(m TrackMetadata, cfg Config) string {
	// Resolve folder template
	folderSegments := parseTemplate(cfg.Folder)
	var folderParts []string
	var currentFolderPart strings.Builder
	// A level the template gives no text for (only spaces, or nothing) is no
	// folder of the path; any other level is one, named "_" if need be.
	endFolder := func() {
		if level := currentFolderPart.String(); strings.TrimSpace(level) != "" {
			folderParts = append(folderParts, folderName(level, cfg))
		}
		currentFolderPart.Reset()
	}

	for _, seg := range folderSegments {
		if seg.isPlaceholder {
			resolved := resolvePlaceholder(seg.value, m, cfg)
			// Apply text transforms to each placeholder value
			resolved = applyTextTransforms(resolved, cfg)
			// Handle conditional: if {year} is empty, skip surrounding separators
			if seg.value == "year" && resolved == "" {
				// Skip the next separator if it follows the year
				continue
			}
			currentFolderPart.WriteString(resolved)
			continue
		}
		// Every "/" of the fixed text ends a folder, whatever stands next to it.
		for i, literal := range strings.Split(seg.value, "/") {
			if i > 0 {
				endFolder()
			}
			// Skip separator literals (like " • ") if previous placeholder was empty
			// This handles cases like "{year} • {album}" when year is empty
			if currentFolderPart.Len() == 0 && strings.TrimSpace(literal) == "•" {
				continue
			}
			currentFolderPart.WriteString(literal)
		}
	}
	endFolder()

	// Handle singles: for single releases with [singles] album, omit album and track number
	isSingle := strings.Contains(strings.ToLower(m.ReleaseType), "single")
	album := m.Album
	if album == "" {
		album = unknownAlbum
	}
	skipAlbumAndTrack := cfg.SinglesHandling && isSingle && album == nonAlbum

	// Resolve filename template
	filenameSegments := parseTemplate(cfg.Filename)
	var filename strings.Builder
	skipNextSeparator := false
	for _, seg := range filenameSegments {
		if seg.isPlaceholder {
			// Skip album and tracknumber for singles
			if skipAlbumAndTrack && (seg.value == "album" || seg.value == "tracknumber") {
				skipNextSeparator = true
				continue
			}
			resolved := resolvePlaceholder(seg.value, m, cfg)
			// Apply text transforms to each placeholder value
			resolved = applyTextTransforms(resolved, cfg)
			filename.WriteString(resolved)
			skipNextSeparator = false
		} else {
			// Skip separator if previous placeholder was skipped
			if skipNextSeparator && (strings.TrimSpace(seg.value) == "•" || strings.TrimSpace(seg.value) == "·") {
				continue
			}
			filename.WriteString(seg.value)
		}
	}
	filenameStr := fileName(filename.String(), cfg)

	// Join folder parts with filename
	folderPath := filepath.Join(folderParts...)
	return filepath.Join(folderPath, filenameStr)
}

// applyTextTransforms applies configured text transformations.
func applyTextTransforms(s string, cfg Config) string {
	if cfg.RemoveFeat {
		s = removeFeatPatterns(s)
	}
	s = normalizeSpaces(s)
	if cfg.EllipsisNormalize {
		s = replace3DotsWithEllipsis(s)
	}
	if cfg.AndToAmpersand {
		s = replaceAndWithAmpersand(s)
	}
	return s
}

// extractReleaseNotes extracts album and track notes from release type strings
func extractReleaseNotes(releaseType, secondaryType string, isVariousArtists bool) (albumNotes, trackNotes string) {
	combinedTypes := strings.ToLower(releaseType + "; " + secondaryType)

	var albumNotesList []string
	var trackNotesList []string

	// Check for album note types
	for _, noteType := range albumNoteTypes {
		if strings.Contains(combinedTypes, noteType) {
			// Skip compilation note for Various Artists releases
			if noteType == "compilation" && isVariousArtists {
				continue
			}
			albumNotesList = append(albumNotesList, noteType)
		}
	}

	// Check for track note types
	for _, noteType := range trackNoteTypes {
		if strings.Contains(combinedTypes, noteType) {
			trackNotesList = append(trackNotesList, noteType)
		}
	}

	albumNotes = strings.Join(albumNotesList, ", ")
	trackNotes = strings.Join(trackNotesList, ", ")
	return
}
