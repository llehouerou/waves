package playerbar

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// State holds everything needed to render the player bar.
type State struct {
	Playing  bool
	Paused   bool
	Track    int
	Title    string
	Artist   string
	Album    string
	Year     int
	Position time.Duration
	Duration time.Duration
}

// Height is the total height of the player bar including borders.
const Height = 3 // top border + content + bottom border

// Render returns the player bar string for the given width.
// Returns empty string if not playing (stopped state).
func Render(s State, width int) string {
	if !s.Playing && !s.Paused {
		return ""
	}

	status := "▶"
	if s.Paused {
		status = "⏸"
	}

	// Right side: position/duration
	right := fmt.Sprintf("%s / %s ", formatDuration(s.Position), formatDuration(s.Duration))
	rightLen := lipgloss.Width(right)

	// Calculate available width (subtract border width of 2)
	innerWidth := max(width-2, 0)

	// Build track info (always shown)
	trackInfo := s.Title
	if s.Track > 0 {
		trackInfo = fmt.Sprintf("%02d - %s", s.Track, s.Title)
	}

	// Build album info
	albumInfo := s.Album
	if albumInfo != "" && s.Year > 0 {
		albumInfo = fmt.Sprintf("%s (%d)", albumInfo, s.Year)
	}

	artistInfo := s.Artist

	// Build combined artist/album: "Artist - Album (Year)"
	var artistAlbumFull, artistOnly string
	if artistInfo != "" {
		artistOnly = artistInfo
		if albumInfo != "" {
			artistAlbumFull = fmt.Sprintf("%s - %s", artistInfo, albumInfo)
		} else {
			artistAlbumFull = artistInfo
		}
	}

	// Calculate minimum width needed: " ▶  trackInfo  right"
	minGap := 2 // minimum gap between sections
	statusPart := " " + status + "  "
	statusLen := lipgloss.Width(statusPart)

	availableForContent := innerWidth - statusLen - rightLen - minGap
	trackLen := lipgloss.Width(trackInfo)
	artistAlbumFullLen := lipgloss.Width(artistAlbumFull)
	artistOnlyLen := lipgloss.Width(artistOnly)

	// Determine what fits: priority is track > artist > artist+album
	var artistPart string
	if artistAlbumFull != "" && artistAlbumFullLen+minGap+trackLen <= availableForContent {
		artistPart = artistAlbumFull
	} else if artistOnly != "" && artistOnlyLen+minGap+trackLen <= availableForContent {
		artistPart = artistOnly
	}

	// Build left content
	var leftParts []string
	if artistPart != "" {
		leftParts = append(leftParts, artistPart)
	}
	leftParts = append(leftParts, trackInfo)

	// Calculate total content width and distribute extra space
	contentWidth := 0
	for _, p := range leftParts {
		contentWidth += lipgloss.Width(p)
	}
	gaps := len(leftParts) // gaps between parts + gap before right

	extraSpace := max(availableForContent-contentWidth, 0)
	gapSize := minGap
	if gaps > 0 && extraSpace > 0 {
		gapSize = (extraSpace / gaps) + minGap
	}

	left := statusPart + strings.Join(leftParts, strings.Repeat(" ", gapSize))
	leftLen := lipgloss.Width(left)

	// Final padding to right-align the timer
	padding := max(innerWidth-leftLen-rightLen, 0)

	content := left + strings.Repeat(" ", padding) + right
	return barStyle.Width(innerWidth).Render(content)
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
