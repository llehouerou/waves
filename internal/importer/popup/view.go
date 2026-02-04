package popup

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// Symbols for status indicators
const (
	completedSymbol = "\u2714" // ✔
	failedSymbol    = "\u2717" // ✗
	progressSymbol  = "\u21E9" // ⇩
	pendingSymbol   = "\u25CB" // ○
)

// Separators
const (
	sepArrow = " \u2192 " // →
)

// Layout thresholds
const (
	// compactWidthThreshold is the width below which compact layout is used.
	// Matches the threshold used in the download results view.
	compactWidthThreshold = 90
)

func titleStyle() lipgloss.Style {
	return styles.T().S().Title
}

func headerStyle() lipgloss.Style {
	return styles.T().S().Muted
}

func stepStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func activeStepStyle() lipgloss.Style {
	return styles.T().S().Playing.Bold(true)
}

func labelStyle() lipgloss.Style {
	return styles.T().S().Muted
}

func valueStyle() lipgloss.Style {
	return styles.T().S().Base
}

func changedStyle() lipgloss.Style {
	return styles.T().S().Warning
}

func successStyle() lipgloss.Style {
	return styles.T().S().Success
}

func errorStyle() lipgloss.Style {
	return styles.T().S().Error
}

func dimStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func selectedStyle() lipgloss.Style {
	return styles.T().S().Cursor
}

// View renders the import popup.
func (m *Model) View() string {
	if m.Width() == 0 || m.Height() == 0 {
		return ""
	}

	var content string
	switch m.state {
	case StateTagPreview:
		content = m.renderTagPreview()
	case StatePathPreview:
		content = m.renderPathPreview()
	case StateImporting:
		content = m.renderImporting()
	case StateComplete:
		content = m.renderComplete()
	}

	return content
}

// innerWidth returns the actual content width accounting for popup border and padding.
func (m *Model) innerWidth() int {
	// Popup has: 2 for border + 4 for padding (2 each side)
	return m.Width() - 8
}

// renderStepIndicator renders the step progress indicator.
func (m *Model) renderStepIndicator() string {
	steps := []string{"Tags", "Paths", "Import"}
	activeIndex := min(int(m.state), 2)

	var parts []string
	for i, step := range steps {
		num := strconv.Itoa(i + 1)
		switch {
		case i == activeIndex:
			parts = append(parts, activeStepStyle().Render(num+" "+step))
		case i < activeIndex:
			parts = append(parts, successStyle().Render(num+" "+step))
		default:
			parts = append(parts, stepStyle().Render(num+" "+step))
		}
	}

	return strings.Join(parts, stepStyle().Render(" \u2192 "))
}

// renderTagPreview renders the tag comparison view.
func (m *Model) renderTagPreview() string {
	// Title
	title := fmt.Sprintf("Import: %s - %s", m.download.MBArtistName, m.download.MBAlbumTitle)

	// Show loading state if refreshing MusicBrainz data
	if m.loadingMB {
		lines := []string{
			titleStyle().Render(title),
			"",
			m.renderStepIndicator(),
			"",
			render.Separator(m.innerWidth()),
			"",
			headerStyle().Render("Refreshing MusicBrainz data..."),
			"",
			dimStyle().Render("Fetching extended tag information from MusicBrainz."),
			dimStyle().Render("This may take a moment due to rate limiting."),
		}
		return strings.Join(lines, "\n")
	}

	// Column widths
	innerWidth := m.innerWidth()
	labelWidth := 14
	valueWidth := (innerWidth - labelWidth - 10) / 2

	// Header row
	header := fmt.Sprintf("%-*s  %-*s  %-*s",
		labelWidth, "Tag",
		valueWidth, "Current",
		valueWidth, "New")

	lines := []string{
		titleStyle().Render(title),
		"",
		m.renderStepIndicator(),
		"",
		render.Separator(innerWidth),
		"",
		headerStyle().Render("Tag Changes Preview"),
		"",
		dimStyle().Render(header),
		dimStyle().Render(strings.Repeat("-", innerWidth)),
	}

	// Tag diffs
	for _, diff := range m.tagDiffs {
		oldVal := render.Truncate(diff.OldValue, valueWidth)
		oldVal = render.Pad(oldVal, valueWidth)

		newVal := render.Truncate(diff.NewValue, valueWidth)
		newVal = render.Pad(newVal, valueWidth)

		label := render.Pad(diff.Field, labelWidth)

		if diff.Changed {
			line := fmt.Sprintf("%s  %s  %s",
				labelStyle().Render(label),
				dimStyle().Render(oldVal),
				changedStyle().Render(newVal))
			lines = append(lines, line)
		} else {
			line := fmt.Sprintf("%s  %s  %s",
				labelStyle().Render(label),
				valueStyle().Render(oldVal),
				valueStyle().Render(newVal))
			lines = append(lines, line)
		}
	}

	// Cover art status and footer
	lines = append(lines,
		"",
		m.renderCoverArtStatus(),
		"",
		dimStyle().Render(fmt.Sprintf("%d files will be retagged", len(m.download.Files))),
		"",
		dimStyle().Render(m.tagPreviewHelpText()),
	)

	return strings.Join(lines, "\n")
}

// tagPreviewHelpText returns the help text for tag preview.
func (m *Model) tagPreviewHelpText() string {
	if !m.coverArtFetched {
		return "[Enter] Continue   [Esc] Cancel   (fetching cover art...)"
	}
	return "[Enter] Continue   [Esc] Cancel"
}

// renderCoverArtStatus renders the cover art status indicator.
func (m *Model) renderCoverArtStatus() string {
	label := labelStyle().Render("Cover Art:    ")

	if !m.coverArtFetched {
		return label + dimStyle().Render("Fetching...")
	}

	if len(m.coverArt) > 0 {
		size := formatBytes(len(m.coverArt))
		return label + successStyle().Render(fmt.Sprintf("%s Found (%s, will be embedded)", completedSymbol, size))
	}

	return label + dimStyle().Render("Not available")
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(b int) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// renderPathPreview renders the file path preview view.
func (m *Model) renderPathPreview() string {
	// Title
	title := fmt.Sprintf("Import: %s - %s", m.download.MBArtistName, m.download.MBAlbumTitle)
	innerWidth := m.innerWidth()

	lines := []string{
		titleStyle().Render(title),
		"",
		m.renderStepIndicator(),
		"",
		render.Separator(innerWidth),
		"",
	}

	// Library selector
	if len(m.librarySources) > 1 {
		lines = append(lines, headerStyle().Render("Destination Library:"))
		for i, source := range m.librarySources {
			prefix := "  "
			if i == m.selectedSource {
				prefix = "> "
				lines = append(lines, selectedStyle().Render(prefix+source))
			} else {
				lines = append(lines, dimStyle().Render(prefix+source))
			}
		}
		lines = append(lines, "")
	} else if len(m.librarySources) == 1 {
		lines = append(lines,
			headerStyle().Render("Destination: ")+valueStyle().Render(m.librarySources[0]),
			"",
		)
	}

	// Path mappings header
	lines = append(lines, headerStyle().Render("File Paths"), "")

	// Check if we should use compact layout
	compact := innerWidth < compactWidthThreshold

	// Calculate available height for file list
	headerLines := len(lines)
	footerLines := 3 // help + empty lines
	availableHeight := m.Height() - headerLines - footerLines - 4

	// In compact mode, each file takes up to 3 lines (1 for name, 2 for wrapped path)
	linesPerFile := 1
	if compact {
		linesPerFile = 3
	}
	maxFiles := availableHeight / linesPerFile

	// Show files with scrolling
	startIdx := m.pathOffset
	endIdx := min(startIdx+maxFiles, len(m.filePaths))

	if compact {
		lines = append(lines, dimStyle().Render(strings.Repeat("-", innerWidth)))
		lines = append(lines, m.renderCompactFilePaths(startIdx, endIdx, innerWidth)...)
	} else {
		// Wide layout: single line per file
		numWidth := 3
		arrowWidth := 4 // " → "
		availWidth := innerWidth - numWidth - arrowWidth - 4
		oldWidth := availWidth * 2 / 5
		newWidth := availWidth * 3 / 5

		header := fmt.Sprintf("%*s  %-*s  %-*s",
			numWidth, "#",
			oldWidth, "Current",
			newWidth, "New Path")
		lines = append(lines,
			dimStyle().Render(header),
			dimStyle().Render(strings.Repeat("-", innerWidth)),
		)

		for i := startIdx; i < endIdx; i++ {
			pm := m.filePaths[i]

			num := fmt.Sprintf("%02d", pm.TrackNum)
			oldName := render.Truncate(pm.Filename, oldWidth)
			oldName = render.Pad(oldName, oldWidth)

			// Show just the relative path part for new path
			newPath := pm.NewPath
			if len(m.librarySources) > 0 {
				newPath = strings.TrimPrefix(newPath, m.librarySources[m.selectedSource])
				newPath = strings.TrimPrefix(newPath, "/")
			}
			newPath = render.Truncate(newPath, newWidth)
			newPath = render.Pad(newPath, newWidth)

			line := fmt.Sprintf("%s  %s%s%s",
				dimStyle().Render(num),
				valueStyle().Render(oldName),
				dimStyle().Render(sepArrow),
				changedStyle().Render(newPath))
			lines = append(lines, line)
		}
	}

	// Scroll indicator
	if len(m.filePaths) > maxFiles {
		scrollInfo := fmt.Sprintf("(%d-%d of %d)", startIdx+1, endIdx, len(m.filePaths))
		lines = append(lines, dimStyle().Render(scrollInfo))
	}

	// Cover art status and help
	helpText := m.pathPreviewHelpText()
	lines = append(lines,
		"",
		m.renderCoverArtStatus(),
		"",
		dimStyle().Render(helpText),
	)

	return strings.Join(lines, "\n")
}

// renderCompactFilePaths renders file paths in compact layout (3 lines per file).
// Line 1: track number + original filename
// Lines 2-3: target path wrapped across up to 2 lines.
func (m *Model) renderCompactFilePaths(startIdx, endIdx, innerWidth int) []string {
	nameWidth := innerWidth - 4 // Leave room for track number
	pathWidth := innerWidth - 6 // Leave room for arrow/continuation indent

	var lines []string
	for i := startIdx; i < endIdx; i++ {
		pm := m.filePaths[i]

		num := fmt.Sprintf("%02d", pm.TrackNum)
		oldName := render.Truncate(pm.Filename, nameWidth)

		// Show just the relative path part for new path
		newPath := pm.NewPath
		if len(m.librarySources) > 0 {
			newPath = strings.TrimPrefix(newPath, m.librarySources[m.selectedSource])
			newPath = strings.TrimPrefix(newPath, "/")
		}

		// Line 1: track number + original filename
		line1 := dimStyle().Render(num) + "  " + valueStyle().Render(oldName)
		lines = append(lines, line1)

		// Lines 2-3: target path (wrapped across up to 2 lines)
		lines = append(lines, m.renderWrappedPath(newPath, pathWidth)...)
	}
	return lines
}

// renderWrappedPath renders a path wrapped across up to 2 lines.
func (m *Model) renderWrappedPath(path string, width int) []string {
	arrow := dimStyle().Render("→")
	pathWidth := runewidth.StringWidth(path)
	if pathWidth <= width {
		// Fits on one line
		line := "    " + arrow + " " + changedStyle().Render(path)
		return []string{line, ""}
	}
	// Wrap to two lines - truncate first line at display width
	line1Path := runewidth.Truncate(path, width, "")
	remaining := strings.TrimPrefix(path, line1Path)
	line1 := "    " + arrow + " " + changedStyle().Render(line1Path)
	line2 := "      " + changedStyle().Render(render.Truncate(remaining, width))
	return []string{line1, line2}
}

// pathPreviewHelpText returns the help text for path preview based on state.
func (m *Model) pathPreviewHelpText() string {
	hasMultipleSources := len(m.librarySources) > 1
	if !m.coverArtFetched {
		if hasMultipleSources {
			return "Waiting for cover art...   [j/k] Select Library   [Esc] Back"
		}
		return "Waiting for cover art...   [Esc] Back"
	}
	if hasMultipleSources {
		return "[Enter] Start Import   [j/k] Select Library   [Esc] Back"
	}
	return "[Enter] Start Import   [Esc] Back"
}

// renderImporting renders the import progress view.
func (m *Model) renderImporting() string {
	// Title
	title := fmt.Sprintf("Import: %s - %s", m.download.MBArtistName, m.download.MBAlbumTitle)
	innerWidth := m.innerWidth()

	lines := make([]string, 0, 12+len(m.importStatus))
	lines = append(lines,
		titleStyle().Render(title),
		"",
		m.renderStepIndicator(),
		"",
		render.Separator(innerWidth),
		"",
		headerStyle().Render("Importing..."),
		"",
	)

	// Progress for each file
	for _, status := range m.importStatus {
		var icon, statusText string
		var style lipgloss.Style

		switch status.Status {
		case StatusComplete:
			icon = completedSymbol
			statusText = "Done"
			style = successStyle()
		case StatusTagging:
			icon = progressSymbol
			statusText = "Tagging..."
			style = changedStyle()
		case StatusMoving:
			icon = progressSymbol
			statusText = "Moving..."
			style = changedStyle()
		case StatusFailed:
			icon = failedSymbol
			statusText = status.Error
			style = errorStyle()
		case StatusPending:
			icon = pendingSymbol
			statusText = "Pending"
			style = dimStyle()
		}

		filename := filepath.Base(strings.ReplaceAll(status.Filename, "\\", "/"))
		filename = render.Truncate(filename, innerWidth/2)

		line := fmt.Sprintf("%s %s  %s",
			style.Render(icon),
			valueStyle().Render(filename),
			style.Render(statusText))
		lines = append(lines, line)
	}

	lines = append(lines, "")

	// Progress count
	completed := 0
	for _, s := range m.importStatus {
		if s.Status == StatusComplete || s.Status == StatusFailed {
			completed++
		}
	}
	progress := fmt.Sprintf("Progress: %d/%d files", completed, len(m.importStatus))
	lines = append(lines,
		dimStyle().Render(progress),
		"",
		dimStyle().Render("[Esc] Close (import continues in background)"),
	)

	return strings.Join(lines, "\n")
}

// renderComplete renders the completion view.
func (m *Model) renderComplete() string {
	// Title
	title := fmt.Sprintf("Import: %s - %s", m.download.MBArtistName, m.download.MBAlbumTitle)
	innerWidth := m.innerWidth()

	lines := []string{
		titleStyle().Render(title),
		"",
		m.renderStepIndicator(),
		"",
		render.Separator(innerWidth),
		"",
	}

	if len(m.failedFiles) == 0 {
		lines = append(lines,
			headerStyle().Render("Import Complete"),
			"",
			successStyle().Render(fmt.Sprintf("%s %d files imported successfully", completedSymbol, m.successCount)),
			successStyle().Render(completedSymbol+" Library index updated"),
		)
	} else {
		lines = append(lines,
			headerStyle().Render("Import Completed with Errors"),
			"",
		)
		if m.successCount > 0 {
			lines = append(lines, successStyle().Render(fmt.Sprintf("%s %d files imported successfully", completedSymbol, m.successCount)))
		}
		lines = append(lines,
			errorStyle().Render(fmt.Sprintf("%s %d files failed", failedSymbol, len(m.failedFiles))),
			"",
		)
		for _, f := range m.failedFiles {
			lines = append(lines, errorStyle().Render(fmt.Sprintf("  - %s: %s", f.Filename, f.Error)))
		}
	}

	lines = append(lines, "")

	// Destination path
	if len(m.filePaths) > 0 && m.filePaths[0].NewPath != "" {
		destDir := filepath.Dir(m.filePaths[0].NewPath)
		lines = append(lines, dimStyle().Render("Destination: ")+valueStyle().Render(destDir))
	}

	lines = append(lines, "", dimStyle().Render("[Enter] Close"))

	return strings.Join(lines, "\n")
}
