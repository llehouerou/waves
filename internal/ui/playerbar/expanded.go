package playerbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/kittyimg"
)

const (
	defaultArtCols = 16
	defaultArtRows = 8
	minArtCols     = 8
	minArtRows     = 4
)

// RenderExpanded renders the expanded player view with album art and metadata.
func RenderExpanded(s State, width int) string {
	innerWidth := max(width-2, 0)
	if innerWidth < 30 {
		// Too narrow, fall back to compact
		return Render(s, width)
	}

	// Determine art dimensions based on available space
	artCols := defaultArtCols
	artRows := defaultArtRows

	// Reduce art size if terminal is narrow
	if innerWidth < 50 {
		artCols = minArtCols
		artRows = minArtRows
	}

	// Build album art (left side)
	var artLines []string
	if len(s.CoverArt) > 0 {
		// Render Kitty image escape sequence on first line
		// The image will span multiple rows visually
		imgSeq := kittyimg.Encode(s.CoverArt, artCols, artRows)
		artLines = append(artLines, imgSeq)
		// Add empty lines for the remaining rows (image takes visual space)
		for i := 1; i < artRows; i++ {
			artLines = append(artLines, strings.Repeat(" ", artCols))
		}
	} else {
		// Use placeholder
		placeholder := kittyimg.Placeholder(artCols, artRows)
		artLines = strings.Split(placeholder, "\n")
	}

	// Build metadata lines (right side)
	metaWidth := innerWidth - artCols - 2 // 2 for gap

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

	// Line 5: Genre (if available and space permits)
	if s.Genre != "" && artRows >= 6 {
		metaLines = append(metaLines, truncate("Genre: "+s.Genre, metaWidth))
	} else {
		metaLines = append(metaLines, "")
	}

	// Line 6: Format info (if available and space permits)
	if s.Format != "" && artRows >= 6 {
		formatInfo := formatAudioInfo(s.Format, s.SampleRate, s.BitDepth)
		metaLines = append(metaLines, truncate(formatInfo, metaWidth))
	} else {
		metaLines = append(metaLines, "")
	}

	// Line 7: Empty
	metaLines = append(metaLines, "")

	// Line 8: Progress bar
	progressBar := RenderProgressBar(s.Position, s.Duration, metaWidth, s.Playing)
	metaLines = append(metaLines, progressBar)

	// Pad metadata lines to match art height
	for len(metaLines) < len(artLines) {
		metaLines = append(metaLines, "")
	}

	// Combine art and metadata side by side
	var contentLines []string
	for i := 0; i < len(artLines) && i < len(metaLines); i++ {
		artLine := artLines[i]
		metaLine := metaLines[i]

		// Pad art line to consistent width (except first line with escape seq)
		if i > 0 || len(s.CoverArt) == 0 {
			artLine = padRight(artLine, artCols)
		}

		line := artLine + "  " + metaLine
		contentLines = append(contentLines, line)
	}

	content := strings.Join(contentLines, "\n")
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

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
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
