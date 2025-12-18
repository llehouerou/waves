package popup

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/render"
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

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	stepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	activeStepStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	changedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")) // orange for changed values

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("34")) // green

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // red

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252"))
)

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
			parts = append(parts, activeStepStyle.Render(num+" "+step))
		case i < activeIndex:
			parts = append(parts, successStyle.Render(num+" "+step))
		default:
			parts = append(parts, stepStyle.Render(num+" "+step))
		}
	}

	return strings.Join(parts, stepStyle.Render(" \u2192 "))
}

// renderTagPreview renders the tag comparison view.
func (m *Model) renderTagPreview() string {
	// Title
	title := fmt.Sprintf("Import: %s - %s", m.download.MBArtistName, m.download.MBAlbumTitle)

	// Show loading state if refreshing MusicBrainz data
	if m.loadingMB {
		lines := []string{
			titleStyle.Render(title),
			"",
			m.renderStepIndicator(),
			"",
			render.Separator(m.innerWidth()),
			"",
			headerStyle.Render("Refreshing MusicBrainz data..."),
			"",
			dimStyle.Render("Fetching extended tag information from MusicBrainz."),
			dimStyle.Render("This may take a moment due to rate limiting."),
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
		titleStyle.Render(title),
		"",
		m.renderStepIndicator(),
		"",
		render.Separator(innerWidth),
		"",
		headerStyle.Render("Tag Changes Preview"),
		"",
		dimStyle.Render(header),
		dimStyle.Render(strings.Repeat("-", innerWidth)),
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
				labelStyle.Render(label),
				dimStyle.Render(oldVal),
				changedStyle.Render(newVal))
			lines = append(lines, line)
		} else {
			line := fmt.Sprintf("%s  %s  %s",
				labelStyle.Render(label),
				valueStyle.Render(oldVal),
				valueStyle.Render(newVal))
			lines = append(lines, line)
		}
	}

	lines = append(lines,
		"",
		dimStyle.Render(fmt.Sprintf("%d files will be retagged", len(m.download.Files))),
		"",
		dimStyle.Render("[Enter] Continue   [Esc] Cancel"),
	)

	return strings.Join(lines, "\n")
}

// renderPathPreview renders the file path preview view.
func (m *Model) renderPathPreview() string {
	// Title
	title := fmt.Sprintf("Import: %s - %s", m.download.MBArtistName, m.download.MBAlbumTitle)
	innerWidth := m.innerWidth()

	lines := []string{
		titleStyle.Render(title),
		"",
		m.renderStepIndicator(),
		"",
		render.Separator(innerWidth),
		"",
	}

	// Library selector
	if len(m.librarySources) > 1 {
		lines = append(lines, headerStyle.Render("Destination Library:"))
		for i, source := range m.librarySources {
			prefix := "  "
			if i == m.selectedSource {
				prefix = "> "
				lines = append(lines, selectedStyle.Render(prefix+source))
			} else {
				lines = append(lines, dimStyle.Render(prefix+source))
			}
		}
		lines = append(lines, "")
	} else if len(m.librarySources) == 1 {
		lines = append(lines,
			headerStyle.Render("Destination: ")+valueStyle.Render(m.librarySources[0]),
			"",
		)
	}

	// Path mappings header
	lines = append(lines, headerStyle.Render("File Paths"), "")

	// Calculate available height for file list
	headerLines := len(lines)
	footerLines := 3 // help + empty lines
	availableHeight := m.Height() - headerLines - footerLines - 4

	// Render file paths
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
		dimStyle.Render(header),
		dimStyle.Render(strings.Repeat("-", innerWidth)),
	)

	// Show files with scrolling
	startIdx := m.pathOffset
	endIdx := min(startIdx+availableHeight, len(m.filePaths))

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
			dimStyle.Render(num),
			valueStyle.Render(oldName),
			dimStyle.Render(sepArrow),
			changedStyle.Render(newPath))
		lines = append(lines, line)
	}

	// Scroll indicator
	if len(m.filePaths) > availableHeight {
		scrollInfo := fmt.Sprintf("(%d-%d of %d)", startIdx+1, endIdx, len(m.filePaths))
		lines = append(lines, dimStyle.Render(scrollInfo))
	}

	lines = append(lines, "")

	// Help
	if len(m.librarySources) > 1 {
		lines = append(lines, dimStyle.Render("[Enter] Start Import   [j/k] Select Library   [Esc] Back"))
	} else {
		lines = append(lines, dimStyle.Render("[Enter] Start Import   [Esc] Back"))
	}

	return strings.Join(lines, "\n")
}

// renderImporting renders the import progress view.
func (m *Model) renderImporting() string {
	// Title
	title := fmt.Sprintf("Import: %s - %s", m.download.MBArtistName, m.download.MBAlbumTitle)
	innerWidth := m.innerWidth()

	lines := []string{
		titleStyle.Render(title),
		"",
		m.renderStepIndicator(),
		"",
		render.Separator(innerWidth),
		"",
		headerStyle.Render("Importing..."),
		"",
	}

	// Progress for each file
	for _, status := range m.importStatus {
		var icon, statusText string
		var style lipgloss.Style

		switch status.Status {
		case StatusComplete:
			icon = completedSymbol
			statusText = "Done"
			style = successStyle
		case StatusTagging:
			icon = progressSymbol
			statusText = "Tagging..."
			style = changedStyle
		case StatusMoving:
			icon = progressSymbol
			statusText = "Moving..."
			style = changedStyle
		case StatusFailed:
			icon = failedSymbol
			statusText = status.Error
			style = errorStyle
		case StatusPending:
			icon = pendingSymbol
			statusText = "Pending"
			style = dimStyle
		}

		filename := filepath.Base(strings.ReplaceAll(status.Filename, "\\", "/"))
		filename = render.Truncate(filename, innerWidth/2)

		line := fmt.Sprintf("%s %s  %s",
			style.Render(icon),
			valueStyle.Render(filename),
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
		dimStyle.Render(progress),
		"",
		dimStyle.Render("[Esc] Close (import continues in background)"),
	)

	return strings.Join(lines, "\n")
}

// renderComplete renders the completion view.
func (m *Model) renderComplete() string {
	// Title
	title := fmt.Sprintf("Import: %s - %s", m.download.MBArtistName, m.download.MBAlbumTitle)
	innerWidth := m.innerWidth()

	lines := []string{
		titleStyle.Render(title),
		"",
		m.renderStepIndicator(),
		"",
		render.Separator(innerWidth),
		"",
	}

	if len(m.failedFiles) == 0 {
		lines = append(lines,
			headerStyle.Render("Import Complete"),
			"",
			successStyle.Render(fmt.Sprintf("%s %d files imported successfully", completedSymbol, m.successCount)),
			successStyle.Render(completedSymbol+" Library index updated"),
		)
	} else {
		lines = append(lines,
			headerStyle.Render("Import Completed with Errors"),
			"",
		)
		if m.successCount > 0 {
			lines = append(lines, successStyle.Render(fmt.Sprintf("%s %d files imported successfully", completedSymbol, m.successCount)))
		}
		lines = append(lines,
			errorStyle.Render(fmt.Sprintf("%s %d files failed", failedSymbol, len(m.failedFiles))),
			"",
		)
		for _, f := range m.failedFiles {
			lines = append(lines, errorStyle.Render(fmt.Sprintf("  - %s: %s", f.Filename, f.Error)))
		}
	}

	lines = append(lines, "")

	// Destination path
	if len(m.filePaths) > 0 && m.filePaths[0].NewPath != "" {
		destDir := filepath.Dir(m.filePaths[0].NewPath)
		lines = append(lines, dimStyle.Render("Destination: ")+valueStyle.Render(destDir))
	}

	lines = append(lines, "", dimStyle.Render("[Enter] Close"))

	return strings.Join(lines, "\n")
}
