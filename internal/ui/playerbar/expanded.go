package playerbar

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/render"
)

// RenderExpanded renders the expanded player view with detailed metadata.
func RenderExpanded(s State, width int) string {
	// Account for border (2) and padding (4)
	innerWidth := max(width-6, 0)
	if innerWidth < ui.MinExpandedWidth {
		// Too narrow, fall back to compact
		return Render(s, width)
	}

	lines := make([]string, 0, 4)

	// Line 1: Title (left) | Genre · Format (right)
	title := s.Title
	if title == "" {
		title = "Unknown Track"
	}

	metaParts := []string{}
	if s.Genre != "" {
		metaParts = append(metaParts, formatGenre(s.Genre))
	}
	if s.Format != "" {
		metaParts = append(metaParts, formatAudioInfo(s.Format, s.SampleRate, s.BitDepth))
	}
	metaLine := strings.Join(metaParts, " · ")

	lines = append(lines, renderRow(
		titleStyle().Render(truncate(title, innerWidth*2/3)),
		metaStyle().Render(truncate(metaLine, innerWidth/3)),
		innerWidth,
	))

	// Line 2: Artist · Album · Year (left) | Track X/Y (right)
	infoParts := []string{}
	artist := s.Artist
	if artist == "" {
		artist = "Unknown Artist"
	}
	infoParts = append(infoParts, artist)
	if s.Album != "" {
		infoParts = append(infoParts, s.Album)
	}
	if s.Year > 0 {
		infoParts = append(infoParts, strconv.Itoa(s.Year))
	}
	infoLine := strings.Join(infoParts, " · ")

	// Track number display (e.g., "Disc 1/2 · 3/12" or just "3/12")
	var trackParts []string
	if s.TotalDiscs > 1 {
		trackParts = append(trackParts, fmt.Sprintf("Disc %d/%d", s.Disc, s.TotalDiscs))
	}
	if s.Track > 0 {
		if s.TotalTracks > 0 {
			trackParts = append(trackParts, fmt.Sprintf("%d/%d", s.Track, s.TotalTracks))
		} else {
			trackParts = append(trackParts, strconv.Itoa(s.Track))
		}
	}
	trackInfo := strings.Join(trackParts, " · ")

	// Line 3: Radio indicator (right-aligned) or empty spacer
	radioLine := ""
	if s.RadioEnabled {
		radioLabel := radioStyle().Render(icons.Radio() + " Radio on")
		radioLine = lipgloss.PlaceHorizontal(innerWidth, lipgloss.Right, radioLabel)
	}

	lines = append(lines,
		renderRow(
			artistStyle().Render(truncate(infoLine, innerWidth*2/3)),
			metaStyle().Render(trackInfo),
			innerWidth,
		),
		radioLine,
	)

	// Line 4: Progress bar (full width)
	progressBar := renderStyledProgressBar(s.Position, s.Duration, innerWidth, s.Playing)
	lines = append(lines, progressBar)

	content := strings.Join(lines, "\n")
	return expandedBarStyle().Width(width - 2).Render(content)
}

// renderRow creates a row with left and right aligned content.
func renderRow(left, right string, width int) string {
	return render.Row(left, right, width)
}

func truncate(s string, maxWidth int) string {
	return render.TruncateEllipsis(s, maxWidth)
}

// formatGenre formats genre for display, replacing ; and , with " / ".
func formatGenre(genre string) string {
	// Replace semicolons and commas with " / "
	result := strings.ReplaceAll(genre, ";", " / ")
	result = strings.ReplaceAll(result, ",", " / ")
	// Clean up any double spaces that might result
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return strings.TrimSpace(result)
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

	return strings.Join(parts, " ")
}

func renderStyledProgressBar(position, duration time.Duration, width int, playing bool) string {
	status := playSymbol
	if !playing {
		status = pauseSymbol
	}

	posStr := formatDuration(position)
	durStr := formatDuration(duration)

	// Calculate space for the bar itself
	// Format: "▶  1:23  ━━━━━───  4:56"
	fixedWidth := lipgloss.Width(status) + 2 + lipgloss.Width(posStr) + 2 + 2 + lipgloss.Width(durStr)
	barWidth := width - fixedWidth

	if barWidth < 5 {
		// Too narrow for bar, just show times
		return status + "  " + progressTimeStyle().Render(posStr+" / "+durStr)
	}

	// Calculate filled portion
	var ratio float64
	if duration > 0 {
		ratio = float64(position) / float64(duration)
	}
	filled := min(int(float64(barWidth)*ratio), barWidth)

	// Use thin bar characters for modern look
	filledBar := progressBarFilled().Render(strings.Repeat("━", filled))
	emptyBar := progressBarEmpty().Render(strings.Repeat("─", barWidth-filled))

	return status + "  " + progressTimeStyle().Render(posStr) + "  " + filledBar + emptyBar + "  " + progressTimeStyle().Render(durStr)
}
