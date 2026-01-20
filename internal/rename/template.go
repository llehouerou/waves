package rename

import (
	"fmt"
	"strconv"
	"strings"
)

// segment represents either a literal string or a placeholder.
type segment struct {
	isPlaceholder bool
	value         string // placeholder name (without braces) or literal text
}

// parseTemplate parses a template string into segments.
// Placeholders are {name}, escaped braces are {{ and }}.
func parseTemplate(template string) []segment {
	if template == "" {
		return nil
	}

	var segments []segment
	var current []rune
	inPlaceholder := false

	runes := []rune(template)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Check for escaped braces
		if r == '{' && i+1 < len(runes) && runes[i+1] == '{' {
			current = append(current, '{')
			i++ // skip next brace
			continue
		}
		if r == '}' && i+1 < len(runes) && runes[i+1] == '}' {
			current = append(current, '}')
			i++ // skip next brace
			continue
		}

		if r == '{' && !inPlaceholder {
			// Start of placeholder - save any accumulated literal
			if len(current) > 0 {
				segments = append(segments, segment{isPlaceholder: false, value: string(current)})
				current = nil
			}
			inPlaceholder = true
			continue
		}

		if r == '}' && inPlaceholder {
			// End of placeholder
			segments = append(segments, segment{isPlaceholder: true, value: string(current)})
			current = nil
			inPlaceholder = false
			continue
		}

		current = append(current, r)
	}

	// Handle remaining content
	if len(current) > 0 {
		segments = append(segments, segment{isPlaceholder: inPlaceholder, value: string(current)})
	}

	return segments
}

// resolvePlaceholder resolves a placeholder name to its value.
// Unknown placeholders are returned as {name} literal.
func resolvePlaceholder(name string, meta TrackMetadata, cfg Config) string {
	switch name {
	case "artist":
		artist := meta.Artist
		if artist == "" {
			artist = meta.AlbumArtist
		}
		if artist == "" {
			artist = unknownArtist
		}
		return artist

	case "albumartist":
		albumArtist := meta.AlbumArtist
		if albumArtist == "" {
			albumArtist = meta.Artist
		}
		if albumArtist == "" {
			albumArtist = unknownArtist
		}

		// Apply VA brackets if enabled
		if cfg.VABrackets {
			isVA := strings.EqualFold(albumArtist, variousArtists)
			isUnknown := strings.EqualFold(albumArtist, unknownArtist)
			if isVA || isUnknown {
				albumArtist = "[" + albumArtist + "]"
			}
		}
		return albumArtist

	case "album":
		album := meta.Album
		if album == "" {
			album = unknownAlbum
		}

		// Check if this is a single with [singles] album
		isSingle := strings.Contains(strings.ToLower(meta.ReleaseType), "single")
		if isSingle && album == nonAlbum {
			return album // Return [singles] as-is
		}

		// Check for unknown album
		isUnknownAlbum := strings.EqualFold(album, unknownAlbum)
		if isUnknownAlbum {
			return "[" + album + "]"
		}

		// Add release type notes (album notes only)
		if cfg.ReleaseTypeNotes {
			isVA := strings.EqualFold(meta.AlbumArtist, variousArtists)
			albumNotes, _ := extractReleaseNotes(meta.ReleaseType, meta.SecondaryReleaseType, isVA)
			if albumNotes != "" {
				album = album + " [" + albumNotes + "]"
			}
		}

		// Add reissue notation
		if cfg.ReissueNotation && meta.OriginalDate != "" && meta.Date != "" {
			origYear := getYear(meta.OriginalDate)
			releaseYear := getYear(meta.Date)
			if origYear != releaseYear && releaseYear != "" {
				album = album + " [" + releaseYear + " reissue]"
			}
		}

		return album

	case "title":
		title := meta.Title
		if title == "" {
			title = unknownTitle
		}

		// Add track notes
		if cfg.ReleaseTypeNotes {
			isVA := strings.EqualFold(meta.AlbumArtist, variousArtists)
			_, trackNotes := extractReleaseNotes(meta.ReleaseType, meta.SecondaryReleaseType, isVA)
			if trackNotes != "" {
				title = title + " [" + trackNotes + "]"
			}
		}

		return title

	case "year":
		year := getYear(meta.OriginalDate)
		if year == "" {
			year = getYear(meta.Date)
		}
		return year

	case "tracknumber":
		if meta.TrackNumber <= 0 {
			return "00"
		}
		if meta.TotalDiscs > 1 && meta.DiscNumber > 0 {
			return fmt.Sprintf("%02d.%02d", meta.DiscNumber, meta.TrackNumber)
		}
		return fmt.Sprintf("%02d", meta.TrackNumber)

	case "discnumber":
		if meta.DiscNumber <= 0 {
			return "1"
		}
		return strconv.Itoa(meta.DiscNumber)

	case "date":
		return meta.Date

	case "originalyear":
		return getYear(meta.OriginalDate)

	default:
		// Unknown placeholder - return as literal
		return "{" + name + "}"
	}
}
