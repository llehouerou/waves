package playerbar

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui/render"
)

// DisplayMode controls the player bar appearance.
type DisplayMode int

const (
	ModeCompact  DisplayMode = iota // Single-line view
	ModeExpanded                    // Detailed view with metadata
)

// State holds everything needed to render the player bar.
type State struct {
	Playing     bool
	Paused      bool
	Track       int
	TotalTracks int
	Disc        int
	TotalDiscs  int
	Title       string
	Artist      string
	Album       string
	Year        int
	Position    time.Duration
	Duration    time.Duration
	DisplayMode DisplayMode
	Genre       string
	Format      string // "MP3" or "FLAC"
	SampleRate  int    // e.g., 44100
	BitDepth    int    // e.g., 16, 24
}

// Height returns the total height of the player bar for the given mode.
func Height(mode DisplayMode) int {
	if mode == ModeExpanded {
		return 6 // 4 content rows + 2 border rows
	}
	return 3 // top border + content + bottom border
}

// NewState constructs a State from player interface and display mode.
// Returns an empty State if player is stopped or has no track info.
func NewState(p player.Interface, mode DisplayMode) State {
	if p.State() == player.Stopped {
		return State{}
	}

	info := p.TrackInfo()
	if info == nil {
		return State{}
	}

	return State{
		Playing:     p.State() == player.Playing,
		Paused:      p.State() == player.Paused,
		Track:       info.Track,
		TotalTracks: info.TotalTracks,
		Disc:        info.Disc,
		TotalDiscs:  info.TotalDiscs,
		Title:       info.Title,
		Artist:      info.Artist,
		Album:       info.Album,
		Year:        info.Year,
		Position:    p.Position(),
		Duration:    p.Duration(),
		DisplayMode: mode,
		Genre:       info.Genre,
		Format:      info.Format,
		SampleRate:  info.SampleRate,
		BitDepth:    info.BitDepth,
	}
}

// Render returns the player bar string for the given width.
// Returns empty string if not playing (stopped state).
func Render(s State, width int) string {
	if !s.Playing && !s.Paused {
		return ""
	}

	if s.DisplayMode == ModeExpanded {
		return RenderExpanded(s, width)
	}

	return renderCompact(s, width)
}

func renderCompact(s State, width int) string {
	// Calculate available width (subtract border and padding)
	innerWidth := max(width-6, 0)

	status := playSymbol
	if s.Paused {
		status = pauseSymbol
	}

	// Build title
	title := s.Title
	if title == "" {
		title = "Unknown Track"
	}

	// Build artist · album · year
	var infoParts []string
	if s.Artist != "" {
		infoParts = append(infoParts, s.Artist)
	}
	if s.Album != "" {
		infoParts = append(infoParts, s.Album)
	}
	if s.Year > 0 {
		infoParts = append(infoParts, strconv.Itoa(s.Year))
	}
	info := strings.Join(infoParts, " · ")

	// Track number (e.g., "Disc 1/2 · 3/12" or just "3/12")
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
	trackNum := strings.Join(trackParts, " · ")

	// Time display
	timeStr := fmt.Sprintf("%s / %s", formatDuration(s.Position), formatDuration(s.Duration))

	// Calculate fixed widths
	separator := "   "
	sepWidth := lipgloss.Width(separator)
	timeWidth := lipgloss.Width(timeStr)
	statusWidth := lipgloss.Width(status + "  ") // status + space before bar
	trackNumWidth := lipgloss.Width(trackNum)

	// Calculate how much space we need for track info
	titleWidth := lipgloss.Width(title)
	infoWidth := lipgloss.Width(info)

	// Reserve minimum space for progress bar (at least 10 chars)
	minBarWidth := 10

	// Available space for title and info (leaving room for trackNum, bar and time)
	trackNumSpace := 0
	if trackNum != "" {
		trackNumSpace = trackNumWidth + sepWidth
	}
	availableForContent := innerWidth - statusWidth - timeWidth - sepWidth*2 - minBarWidth - trackNumSpace

	var styledTitle, styledInfo string
	var usedContentWidth int

	switch {
	case titleWidth+sepWidth+infoWidth <= availableForContent:
		// Everything fits
		styledTitle = titleStyle().Render(title)
		styledInfo = artistStyle().Render(info)
		usedContentWidth = titleWidth + sepWidth + infoWidth
	case titleWidth+sepWidth <= availableForContent && info != "":
		// Truncate info
		maxInfo := availableForContent - titleWidth - sepWidth
		styledTitle = titleStyle().Render(title)
		styledInfo = artistStyle().Render(truncateCompact(info, maxInfo))
		usedContentWidth = titleWidth + sepWidth + maxInfo
	default:
		// Truncate title, no info
		maxTitle := max(availableForContent, 10)
		styledTitle = titleStyle().Render(truncateCompact(title, maxTitle))
		styledInfo = ""
		usedContentWidth = min(titleWidth, maxTitle)
	}

	// Calculate progress bar width (use remaining space)
	barWidth := max(innerWidth-usedContentWidth-trackNumSpace-statusWidth-timeWidth-sepWidth*2, 5)

	// Build progress bar
	var ratio float64
	if s.Duration > 0 {
		ratio = float64(s.Position) / float64(s.Duration)
	}
	filled := min(int(float64(barWidth)*ratio), barWidth)
	filledBar := progressBarFilled().Render(strings.Repeat("━", filled))
	emptyBar := progressBarEmpty().Render(strings.Repeat("─", barWidth-filled))

	// Build the line: Title   Info   3/12   ▶ ━━━───   1:23 / 3:58
	var content strings.Builder
	content.WriteString(styledTitle)
	if styledInfo != "" {
		content.WriteString(separator)
		content.WriteString(styledInfo)
	}
	if trackNum != "" {
		content.WriteString(separator)
		content.WriteString(metaStyle().Render(trackNum))
	}
	content.WriteString(separator)
	content.WriteString(status)
	content.WriteString("  ")
	content.WriteString(filledBar)
	content.WriteString(emptyBar)
	content.WriteString(separator)
	content.WriteString(progressTimeStyle().Render(timeStr))

	return barStyle().Padding(0, 2).Width(width - 2).Render(content.String())
}

func truncateCompact(s string, maxWidth int) string {
	return render.TruncateEllipsis(s, maxWidth)
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
