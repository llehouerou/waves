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
	failedSymbol    = "\u2717" // ✗
	downloadingIcon = "\u21E9" // ⇩
	pendingIcon     = "\u25CB" // ○
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
)

// View renders the downloads view.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	innerWidth := m.width - ui.BorderHeight
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
			for _, f := range d.Files {
				if lineIdx >= m.offset+listHeight {
					break
				}
				if lineIdx >= m.offset {
					line := m.renderFileLine(&f, innerWidth)
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
		statusText = fmt.Sprintf("[%s Completed]", completedSymbol)
		statusStyle = completedStyle
	case downloads.StatusDownloading:
		statusText = fmt.Sprintf("[%s %d/%d files, %.0f%%]", downloadingIcon, completed, total, percent)
		statusStyle = downloadingStyle
	case downloads.StatusFailed:
		failedCount := 0
		for _, f := range d.Files {
			if f.Status == downloads.StatusFailed {
				failedCount++
			}
		}
		statusText = fmt.Sprintf("[%s Failed %d files]", failedSymbol, failedCount)
		statusStyle = failedStyle
	case downloads.StatusPending:
		statusText = fmt.Sprintf("[%s Pending]", pendingIcon)
		statusStyle = pendingStyle
	}

	// Main content: Artist - Album
	albumInfo := fmt.Sprintf("%s - %s", d.MBArtistName, d.MBAlbumTitle)
	if d.MBReleaseYear != "" {
		albumInfo = fmt.Sprintf("%s (%s)", albumInfo, d.MBReleaseYear)
	}

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

// renderFileLine renders a single file entry within an expanded download.
func (m Model) renderFileLine(f *downloads.DownloadFile, width int) string {
	// Indent for file entries
	indent := "    "

	// Status icon
	var icon string
	var style lipgloss.Style
	switch f.Status {
	case downloads.StatusCompleted:
		icon = completedSymbol
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

	// File name (just the base name)
	filename := filepath.Base(f.Filename)

	// Progress for downloading files
	progress := ""
	if f.Status == downloads.StatusDownloading && f.Size > 0 {
		percent := float64(f.BytesRead) / float64(f.Size) * 100
		progress = fmt.Sprintf(" %.0f%%", percent)
	}

	// Available width for filename
	indentWidth := lipgloss.Width(indent)
	iconWidth := 2 // icon + space
	progressWidth := lipgloss.Width(progress)
	filenameWidth := width - indentWidth - iconWidth - progressWidth

	filename = render.Truncate(filename, filenameWidth)
	filename = render.Pad(filename, filenameWidth)

	line := indent + icon + " " + filename + progress

	return style.Render(line)
}
