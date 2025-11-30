package playerbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/kittyimg"
)

const (
	artCols     = 20
	artRows     = 10
	contentRows = 10 // Must match Height(ModeExpanded) - 2 for borders
)

// RenderExpanded renders the expanded player view with album art and metadata.
func RenderExpanded(s State, width int) string {
	innerWidth := max(width-2, 0)
	if innerWidth < 40 {
		// Too narrow, fall back to compact
		return Render(s, width)
	}

	metaWidth := innerWidth - artCols - 2 // 2 for gap between art and metadata

	// Build the metadata block (right side)
	var metaLines []string

	// Line 1: Artist
	artist := s.Artist
	if artist == "" {
		artist = "Unknown Artist"
	}
	metaLines = append(metaLines, truncate(artist, metaWidth))

	// Line 2: Album (Year)
	album := s.Album
	if album == "" {
		album = "Unknown Album"
	}
	if s.Year > 0 {
		album = fmt.Sprintf("%s (%d)", album, s.Year)
	}
	// Line 3: Empty
	metaLines = append(metaLines, truncate(album, metaWidth), "")

	// Line 4: Track title
	title := s.Title
	if s.Track > 0 {
		title = fmt.Sprintf("%02d - %s", s.Track, s.Title)
	}
	metaLines = append(metaLines, truncate(title, metaWidth))

	// Line 5: Genre
	if s.Genre != "" {
		metaLines = append(metaLines, truncate("Genre: "+s.Genre, metaWidth))
	} else {
		metaLines = append(metaLines, "")
	}

	// Line 6: Format info
	if s.Format != "" {
		formatInfo := formatAudioInfo(s.Format, s.SampleRate, s.BitDepth)
		metaLines = append(metaLines, truncate(formatInfo, metaWidth))
	} else {
		metaLines = append(metaLines, "")
	}

	// Lines 7-8: Empty
	metaLines = append(metaLines, "", "")

	// Line 9: Progress bar, Line 10: Empty (padding to fill height)
	progressBar := RenderProgressBar(s.Position, s.Duration, metaWidth, s.Playing)
	metaLines = append(metaLines, progressBar, "")

	// Ensure we have exactly contentRows lines
	for len(metaLines) < contentRows {
		metaLines = append(metaLines, "")
	}
	metaLines = metaLines[:contentRows]

	// Build the art placeholder block (left side) - just spaces
	// The actual image is rendered via escape sequence prepended later
	artBlock := strings.Repeat(strings.Repeat(" ", artCols)+"\n", contentRows-1) + strings.Repeat(" ", artCols)

	// Build metadata block
	metaBlock := strings.Join(metaLines, "\n")

	// Join art and metadata horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, artBlock, "  ", metaBlock)

	// Render with border
	rendered := expandedBarStyle.Width(innerWidth).Render(content)

	// Prepend the image escape sequence (renders image at cursor, then content follows)
	if len(s.CoverArt) > 0 {
		imgSeq := kittyimg.Encode(s.CoverArt, artCols, artRows)
		return imgSeq + rendered
	}

	// No cover art - render placeholder instead
	placeholder := kittyimg.Placeholder(artCols, artRows)
	placeholderLines := strings.Split(placeholder, "\n")

	// We need to overlay the placeholder on the art area
	// For now, just include it in the content
	lines := strings.Split(content, "\n")
	for i := 0; i < len(placeholderLines) && i < len(lines); i++ {
		lines[i] = placeholderLines[i] + lines[i][artCols:]
	}
	content = strings.Join(lines, "\n")

	return expandedBarStyle.Width(innerWidth).Render(content)
}

func truncate(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	// Truncate with ellipsis
	for lipgloss.Width(s) > maxWidth-1 && s != "" {
		s = s[:len(s)-1]
	}
	return s + "…"
}

func formatAudioInfo(format string, sampleRate, bitDepth int) string {
	var parts []string
	parts = append(parts, format)

	if sampleRate > 0 {
		khz := float64(sampleRate) / 1000.0
		if khz == float64(int(khz)) {
			parts = append(parts, fmt.Sprintf("%d kHz", int(khz)))
		} else {
			parts = append(parts, fmt.Sprintf("%.1f kHz", khz))
		}
	}

	if bitDepth > 0 && format != "MP3" {
		parts = append(parts, fmt.Sprintf("%d-bit", bitDepth))
	}

	return strings.Join(parts, " · ")
}
