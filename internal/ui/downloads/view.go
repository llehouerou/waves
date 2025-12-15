package downloads

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// Symbols for status indicators
const (
	expandedSymbol  = "\u25BC" // ▼
	collapsedSymbol = "\u25B6" // ▶
	completedSymbol = "\u2713" // ✓
	verifiedSymbol  = "\u2714" // ✔ (heavy check - verified on disk)
	failedSymbol    = "\u2717" // ✗
	downloadingIcon = "\u21E9" // ⇩
	pendingIcon     = "\u25CB" // ○
)

// Separators matching the renamer style
const (
	sepBullet = " • " // U+2022 Bullet - between major elements
	sepDot    = " · " // U+00B7 Middle Dot - between track number and title
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))

	downloadStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252"))

	completedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("34")) // green

	downloadingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")) // cyan/blue

	failedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // red

	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")) // dim

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)

	mbTrackStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")) // cyan for matched track
)

// View renders the downloads view.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	innerWidth := m.width - ui.BorderHeight
	innerHeight := m.height - ui.BorderHeight // Account for top/bottom border
	listHeight := m.listHeight()

	// Header
	header := m.renderHeader(innerWidth)

	// Separator
	separator := render.Separator(innerWidth)

	// Download list
	downloadList := m.renderDownloadList(innerWidth, listHeight)

	content := header + "\n" + separator + "\n" + downloadList

	return styles.PanelStyle(m.focused).
		Width(innerWidth).
		Height(innerHeight).
		Render(content)
}

// renderHeader renders the downloads header.
func (m Model) renderHeader(innerWidth int) string {
	headerText := m.buildHeaderText()
	headerText = render.TruncateAndPad(headerText, innerWidth)
	return headerStyle.Render(headerText)
}

// buildHeaderText constructs the header text with status counts.
func (m Model) buildHeaderText() string {
	if len(m.downloads) == 0 {
		return "Downloads"
	}

	// Count by status
	var pending, downloading, completed, failed int
	for i := range m.downloads {
		switch m.downloads[i].Status {
		case downloads.StatusPending:
			pending++
		case downloads.StatusDownloading:
			downloading++
		case downloads.StatusCompleted:
			completed++
		case downloads.StatusFailed:
			failed++
		}
	}

	var parts []string
	if downloading > 0 {
		parts = append(parts, fmt.Sprintf("%d active", downloading))
	}
	if pending > 0 {
		parts = append(parts, fmt.Sprintf("%d pending", pending))
	}
	if completed > 0 {
		parts = append(parts, fmt.Sprintf("%d done", completed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}

	if len(parts) > 0 {
		return fmt.Sprintf("Downloads (%s)", strings.Join(parts, ", "))
	}
	return fmt.Sprintf("Downloads (%d)", len(m.downloads))
}

// renderDownloadList renders the list of downloads.
func (m Model) renderDownloadList(innerWidth, listHeight int) string {
	if len(m.downloads) == 0 {
		return m.renderEmptyState(innerWidth, listHeight)
	}

	lines := make([]string, 0, listHeight)
	lineIdx := 0

	for i := range m.downloads {
		if lineIdx >= m.offset+listHeight {
			break
		}

		d := &m.downloads[i]

		// Render download header line
		if lineIdx >= m.offset {
			line := m.renderDownloadLine(d, i, innerWidth)
			lines = append(lines, line)
		}
		lineIdx++

		// Render expanded files if applicable
		if m.isExpanded(d.ID) {
			for fileIdx, f := range d.Files {
				if lineIdx >= m.offset+listHeight {
					break
				}
				if lineIdx >= m.offset {
					line := m.renderFileLine(d, &f, fileIdx, innerWidth)
					lines = append(lines, line)
				}
				lineIdx++
			}
		}
	}

	// Fill remaining lines
	for len(lines) < listHeight {
		lines = append(lines, render.EmptyLine(innerWidth))
	}

	return strings.Join(lines, "\n")
}

// renderEmptyState renders the empty download list view.
func (m Model) renderEmptyState(innerWidth, listHeight int) string {
	emptyMsg := "No downloads"
	centerLine := listHeight / 2
	lines := make([]string, listHeight)

	for i := range lines {
		if i == centerLine {
			centered := lipgloss.NewStyle().Width(innerWidth).Align(lipgloss.Center).Render(emptyMsg)
			lines[i] = emptyStyle.Render(centered)
		} else {
			lines[i] = render.EmptyLine(innerWidth)
		}
	}

	return strings.Join(lines, "\n")
}

// renderDownloadLine renders a single download entry.
func (m Model) renderDownloadLine(d *downloads.Download, idx, width int) string {
	isCursor := idx == m.cursor && m.focused

	// Prefix: ▶ or ▼ based on expanded state
	prefix := collapsedSymbol + " "
	if m.isExpanded(d.ID) {
		prefix = expandedSymbol + " "
	}

	// Status icon and text
	completed, total, percent := d.Progress()
	var statusText string
	var statusStyle lipgloss.Style

	switch d.Status {
	case downloads.StatusCompleted:
		statusText = fmt.Sprintf("[%s]", completedSymbol)
		statusStyle = completedStyle
	case downloads.StatusDownloading:
		statusText = fmt.Sprintf("[%s %.0f%%]", downloadingIcon, percent)
		statusStyle = downloadingStyle
	case downloads.StatusFailed:
		failedCount := 0
		for _, f := range d.Files {
			if f.Status == downloads.StatusFailed {
				failedCount++
			}
		}
		statusText = fmt.Sprintf("[%s %d]", failedSymbol, failedCount)
		statusStyle = failedStyle
	case downloads.StatusPending:
		statusText = fmt.Sprintf("[%s]", pendingIcon)
		statusStyle = pendingStyle
	}

	// Build content in renamer style: Artist • Year • Album • Tracks
	var parts []string
	parts = append(parts, d.MBArtistName)

	// Add year from release details if available, otherwise from release year
	year := ""
	switch {
	case d.MBReleaseDetails != nil && d.MBReleaseDetails.Date != "":
		year = extractYear(d.MBReleaseDetails.Date)
	case d.MBReleaseGroup != nil && d.MBReleaseGroup.FirstRelease != "":
		year = extractYear(d.MBReleaseGroup.FirstRelease)
	case d.MBReleaseYear != "":
		year = extractYear(d.MBReleaseYear)
	}
	if year != "" {
		parts = append(parts, year)
	}

	parts = append(parts, d.MBAlbumTitle)

	// Track count info
	trackInfo := fmt.Sprintf("%d/%d", completed, total)
	if d.MBReleaseDetails != nil && len(d.MBReleaseDetails.Tracks) > 0 {
		trackInfo = fmt.Sprintf("%d/%d tracks", completed, len(d.MBReleaseDetails.Tracks))
	}
	parts = append(parts, trackInfo)

	albumInfo := strings.Join(parts, sepBullet)

	// Calculate available width
	prefixWidth := 2 // "▶ "
	statusWidth := lipgloss.Width(statusText)
	contentWidth := width - prefixWidth - statusWidth - 1 // -1 for space before status

	// Truncate album info to fit
	albumInfo = render.Truncate(albumInfo, contentWidth)
	albumInfo = render.Pad(albumInfo, contentWidth)

	line := prefix + albumInfo + " " + statusStyle.Render(statusText)

	// Apply cursor style if selected
	if isCursor {
		return cursorStyle.Width(width).Render(line)
	}
	return downloadStyle.Render(line)
}

// extractYear returns the first 4 characters of a date string (YYYY).
func extractYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return date
}

// renderFileLine renders a single file entry within an expanded download.
func (m Model) renderFileLine(d *downloads.Download, f *downloads.DownloadFile, fileIdx, width int) string {
	// Indent for file entries
	indent := "    "

	// Status icon
	var icon string
	var style lipgloss.Style
	switch f.Status {
	case downloads.StatusCompleted:
		if f.VerifiedOnDisk {
			icon = verifiedSymbol // ✔ heavy check for verified on disk
		} else {
			icon = completedSymbol // ✓ regular check for slskd completed
		}
		style = completedStyle
	case downloads.StatusDownloading:
		icon = downloadingIcon
		style = downloadingStyle
	case downloads.StatusFailed:
		icon = failedSymbol
		style = failedStyle
	default:
		icon = pendingIcon
		style = pendingStyle
	}

	// Try to match with MusicBrainz track
	var trackInfo string
	if d.MBReleaseDetails != nil && fileIdx < len(d.MBReleaseDetails.Tracks) {
		track := d.MBReleaseDetails.Tracks[fileIdx]
		// Format: 01 · Track Title
		trackInfo = fmt.Sprintf("%02d%s%s", track.Position, sepDot, track.Title)
	}

	// File name (just the base name)
	filename := filepath.Base(f.Filename)

	// Progress for downloading files
	progress := ""
	if f.Status == downloads.StatusDownloading && f.Size > 0 {
		percent := float64(f.BytesRead) / float64(f.Size) * 100
		progress = fmt.Sprintf(" %.0f%%", percent)
	}

	// Build the line content
	var content string
	if trackInfo != "" {
		// Show: filename → 01 · Track Title
		arrow := " → "
		// Calculate available widths
		indentWidth := lipgloss.Width(indent)
		iconWidth := 2 // icon + space
		arrowWidth := lipgloss.Width(arrow)
		progressWidth := lipgloss.Width(progress)
		availableWidth := width - indentWidth - iconWidth - progressWidth

		// Split available width: ~40% for filename, ~60% for track info
		filenameWidth := availableWidth * 2 / 5
		trackWidth := availableWidth - filenameWidth - arrowWidth

		filename = render.Truncate(filename, filenameWidth)
		filename = render.Pad(filename, filenameWidth)

		trackInfo = render.Truncate(trackInfo, trackWidth)
		trackInfo = render.Pad(trackInfo, trackWidth)

		content = filename + arrow + mbTrackStyle.Render(trackInfo) + progress
	} else {
		// No MB data - just show filename
		indentWidth := lipgloss.Width(indent)
		iconWidth := 2 // icon + space
		progressWidth := lipgloss.Width(progress)
		filenameWidth := width - indentWidth - iconWidth - progressWidth

		filename = render.Truncate(filename, filenameWidth)
		filename = render.Pad(filename, filenameWidth)

		content = filename + progress
	}

	line := indent + icon + " " + content

	return style.Render(line)
}
