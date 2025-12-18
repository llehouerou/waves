package retag

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/ui/render"
)

// Symbols for status indicators
const (
	completedSymbol = "\u2714" // ✔
	failedSymbol    = "\u2717" // ✗
	progressSymbol  = "\u21E9" // ⇩
	pendingSymbol   = "\u25CB" // ○
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	changedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")) // orange

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("34")) // green

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // red

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252"))

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	typeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))
)

// View renders the retag popup.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var content string
	switch m.state {
	case StateLoading:
		content = m.renderLoading("Reading tags from files...")
	case StateSearching:
		content = m.renderLoading(m.statusMsg)
	case StateReleaseGroupResults:
		content = m.renderReleaseGroupResults()
	case StateReleaseLoading:
		content = m.renderLoading("Loading releases...")
	case StateReleaseResults:
		content = m.renderReleaseResults()
	case StateReleaseDetailsLoading:
		content = m.renderLoading("Loading release details...")
	case StateTagPreview:
		content = m.renderTagPreview()
	case StateRetagging:
		content = m.renderRetagging()
	case StateComplete:
		content = m.renderComplete()
	}

	return content
}

// innerWidth returns the actual content width accounting for popup border and padding.
func (m *Model) innerWidth() int {
	return m.width - 8
}

// renderSearchMethodInfo renders information about how the search was performed.
func (m *Model) renderSearchMethodInfo() []string {
	var lines []string

	// Show which MusicBrainz IDs were found in tags
	if m.foundMBReleaseID != "" {
		lines = append(lines, dimStyle.Render("Release ID: ")+valueStyle.Render(m.foundMBReleaseID))
	}
	if m.foundMBReleaseGroupID != "" {
		lines = append(lines, dimStyle.Render("Release Group ID: ")+valueStyle.Render(m.foundMBReleaseGroupID))
	}
	if m.foundMBArtistID != "" {
		lines = append(lines, dimStyle.Render("Artist ID: ")+valueStyle.Render(m.foundMBArtistID))
	}

	// Show search method
	if m.searchMethod != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, dimStyle.Render("Method: ")+headerStyle.Render(m.searchMethod))
	}

	if len(lines) > 0 {
		lines = append(lines, "")
	}

	return lines
}

// renderLoading renders a loading state.
func (m *Model) renderLoading(message string) string {
	title := fmt.Sprintf("Retag: %s - %s", m.albumArtist, m.albumName)
	innerWidth := m.innerWidth()

	lines := []string{
		titleStyle.Render(title),
		"",
		render.Separator(innerWidth),
		"",
		headerStyle.Render(message),
		"",
		dimStyle.Render("[Esc] Cancel"),
	}

	if m.errorMsg != "" {
		lines = append(lines[:4], errorStyle.Render("Error: "+m.errorMsg))
		lines = append(lines, "", dimStyle.Render("[Esc] Close"))
	}

	return strings.Join(lines, "\n")
}

// renderReleaseGroupResults renders the release group selection view.
func (m *Model) renderReleaseGroupResults() string {
	title := fmt.Sprintf("Retag: %s - %s", m.albumArtist, m.albumName)
	innerWidth := m.innerWidth()

	lines := []string{
		titleStyle.Render(title),
		"",
	}

	// Show search method info
	lines = append(lines, m.renderSearchMethodInfo()...)

	lines = append(lines,
		render.Separator(innerWidth),
		"",
	)

	// Search input if in search mode
	if m.searchMode {
		lines = append(lines,
			headerStyle.Render("Search: ")+m.searchInput.View(),
			"",
		)
	}

	if m.errorMsg != "" {
		lines = append(lines, errorStyle.Render("Error: "+m.errorMsg), "")
	}

	if len(m.releaseGroups) == 0 {
		lines = append(lines,
			dimStyle.Render("No release groups found"),
			"",
			dimStyle.Render("[/] Search   [Esc] Close"),
		)
		return strings.Join(lines, "\n")
	}

	lines = append(lines, headerStyle.Render("Select a release group:"), "")

	// Render release groups
	maxVisible := max(m.height-12, 5)
	start, end := m.releaseGroupCursor.VisibleRange(len(m.releaseGroups), maxVisible)
	cursorPos := m.releaseGroupCursor.Pos()

	for i := start; i < end; i++ {
		rg := &m.releaseGroups[i]
		line := m.formatReleaseGroup(rg)

		if i == cursorPos {
			lines = append(lines, cursorStyle.Render("> ")+selectedStyle.Render(line))
		} else {
			lines = append(lines, "  "+line)
		}
	}

	// Scroll indicator
	if len(m.releaseGroups) > maxVisible {
		scrollInfo := fmt.Sprintf("(%d-%d of %d)", start+1, end, len(m.releaseGroups))
		lines = append(lines, "", dimStyle.Render(scrollInfo))
	}

	lines = append(lines, "", dimStyle.Render("[Enter] Select   [/] Search   [Esc] Close"))

	return strings.Join(lines, "\n")
}

// formatReleaseGroup formats a single release group.
func (m *Model) formatReleaseGroup(rg *musicbrainz.ReleaseGroup) string {
	// Always show artist first for clarity
	var artistPart string
	if rg.Artist != "" {
		if strings.EqualFold(rg.Artist, m.albumArtist) {
			// Matching artist - show in green
			artistPart = successStyle.Render(rg.Artist)
		} else {
			// Different artist - show dimmed
			artistPart = dimStyle.Render(rg.Artist)
		}
	}

	parts := []string{}
	if artistPart != "" {
		parts = append(parts, artistPart+" - ")
	}
	parts = append(parts, rg.Title)

	if rg.FirstRelease != "" {
		year := rg.FirstRelease
		if len(year) > 4 {
			year = year[:4]
		}
		parts = append(parts, fmt.Sprintf(" (%s)", year))
	}

	if rg.PrimaryType != "" {
		parts = append(parts, " "+typeStyle.Render(fmt.Sprintf("[%s]", rg.PrimaryType)))
	}

	if len(rg.SecondaryTypes) > 0 {
		parts = append(parts, " "+dimStyle.Render("+"+strings.Join(rg.SecondaryTypes, "+")))
	}

	return strings.Join(parts, "")
}

// renderReleaseResults renders the release selection view.
func (m *Model) renderReleaseResults() string {
	title := fmt.Sprintf("Retag: %s - %s", m.albumArtist, m.albumName)
	innerWidth := m.innerWidth()

	lines := []string{
		titleStyle.Render(title),
		"",
	}

	// Show selected release group
	if m.selectedReleaseGroup != nil {
		lines = append(lines,
			dimStyle.Render("Release Group: ")+valueStyle.Render(m.selectedReleaseGroup.Title),
		)
	}

	lines = append(lines,
		render.Separator(innerWidth),
		"",
	)

	if m.errorMsg != "" {
		lines = append(lines, errorStyle.Render("Error: "+m.errorMsg), "")
	}

	if len(m.releases) == 0 {
		lines = append(lines,
			dimStyle.Render("No releases found"),
			"",
			dimStyle.Render("[Backspace] Back   [Esc] Close"),
		)
		return strings.Join(lines, "\n")
	}

	lines = append(lines, headerStyle.Render("Select a release:"), "")

	// Render releases
	maxVisible := max(m.height-14, 5)
	start, end := m.releaseCursor.VisibleRange(len(m.releases), maxVisible)
	cursorPos := m.releaseCursor.Pos()

	for i := start; i < end; i++ {
		r := &m.releases[i]
		line := m.formatRelease(r)

		if i == cursorPos {
			lines = append(lines, cursorStyle.Render("> ")+selectedStyle.Render(line))
		} else {
			lines = append(lines, "  "+line)
		}
	}

	// Scroll indicator
	if len(m.releases) > maxVisible {
		scrollInfo := fmt.Sprintf("(%d-%d of %d)", start+1, end, len(m.releases))
		lines = append(lines, "", dimStyle.Render(scrollInfo))
	}

	lines = append(lines, "", dimStyle.Render("[Enter] Select   [Backspace] Back   [Esc] Close"))

	return strings.Join(lines, "\n")
}

// formatRelease formats a single release.
func (m *Model) formatRelease(r *musicbrainz.Release) string {
	parts := []string{r.Title}

	localTrackCount := len(m.trackPaths)
	if r.TrackCount > 0 {
		if r.TrackCount == localTrackCount {
			// Track count matches - show in green with checkmark
			parts = append(parts, successStyle.Render(fmt.Sprintf("[%d tracks %s]", r.TrackCount, completedSymbol)))
		} else {
			// Track count doesn't match
			parts = append(parts, typeStyle.Render(fmt.Sprintf("[%d tracks]", r.TrackCount)))
		}
	}

	if r.Date != "" {
		year := r.Date
		if len(year) > 4 {
			year = year[:4]
		}
		parts = append(parts, fmt.Sprintf("(%s)", year))
	}

	if r.Country != "" {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("[%s]", r.Country)))
	}

	if r.Formats != "" {
		parts = append(parts, dimStyle.Render(r.Formats))
	}

	return strings.Join(parts, " ")
}

// renderTagPreview renders the tag comparison view.
func (m *Model) renderTagPreview() string {
	title := fmt.Sprintf("Retag: %s - %s", m.albumArtist, m.albumName)
	innerWidth := m.innerWidth()

	// Column widths
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
	}

	// Show selected release info
	if m.releaseDetails != nil {
		lines = append(lines,
			dimStyle.Render("Release: ")+valueStyle.Render(m.releaseDetails.Title),
		)
		if m.releaseDetails.Date != "" {
			lines = append(lines,
				dimStyle.Render("Date: ")+valueStyle.Render(m.releaseDetails.Date),
			)
		}
	}

	lines = append(lines,
		render.Separator(innerWidth),
		"",
		headerStyle.Render("Tag Changes Preview"),
		"",
		dimStyle.Render(header),
		dimStyle.Render(strings.Repeat("-", innerWidth)),
	)

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
		dimStyle.Render(fmt.Sprintf("%d files will be retagged", len(m.trackPaths))),
		"",
		dimStyle.Render("[Enter] Retag   [Backspace] Back   [Esc] Cancel"),
	)

	return strings.Join(lines, "\n")
}

// renderRetagging renders the retag progress view.
func (m *Model) renderRetagging() string {
	title := fmt.Sprintf("Retag: %s - %s", m.albumArtist, m.albumName)
	innerWidth := m.innerWidth()

	lines := []string{
		titleStyle.Render(title),
		"",
		render.Separator(innerWidth),
		"",
		headerStyle.Render("Retagging..."),
		"",
	}

	if m.statusMsg != "" {
		lines = append(lines, dimStyle.Render(m.statusMsg), "")
	}

	// Progress for each file
	maxVisible := max(m.height-14, 5)
	visibleCount := min(len(m.retagStatus), maxVisible)

	for i := range visibleCount {
		status := m.retagStatus[i]
		var icon, statusText string
		var style lipgloss.Style

		switch status.Status {
		case StatusComplete:
			icon = completedSymbol
			statusText = "Done"
			style = successStyle
		case StatusRetagging:
			icon = progressSymbol
			statusText = "Retagging..."
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

		filename := filepath.Base(status.Filename)
		filename = render.Truncate(filename, innerWidth/2)

		line := fmt.Sprintf("%s %s  %s",
			style.Render(icon),
			valueStyle.Render(filename),
			style.Render(statusText))
		lines = append(lines, line)
	}

	if len(m.retagStatus) > maxVisible {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("... and %d more", len(m.retagStatus)-maxVisible)))
	}

	lines = append(lines, "")

	// Progress count
	completed := 0
	for _, s := range m.retagStatus {
		if s.Status == StatusComplete || s.Status == StatusFailed {
			completed++
		}
	}
	progress := fmt.Sprintf("Progress: %d/%d files", completed, len(m.retagStatus))
	lines = append(lines,
		dimStyle.Render(progress),
		"",
		dimStyle.Render("[Esc] Close (retag continues in background)"),
	)

	return strings.Join(lines, "\n")
}

// renderComplete renders the completion view.
func (m *Model) renderComplete() string {
	title := fmt.Sprintf("Retag: %s - %s", m.albumArtist, m.albumName)
	innerWidth := m.innerWidth()

	lines := []string{
		titleStyle.Render(title),
		"",
		render.Separator(innerWidth),
		"",
	}

	if len(m.failedFiles) == 0 {
		lines = append(lines,
			headerStyle.Render("Retag Complete"),
			"",
			successStyle.Render(fmt.Sprintf("%s %d files retagged successfully", completedSymbol, m.successCount)),
			successStyle.Render(completedSymbol+" Library index updated"),
		)
	} else {
		lines = append(lines,
			headerStyle.Render("Retag Completed with Errors"),
			"",
		)
		if m.successCount > 0 {
			lines = append(lines, successStyle.Render(fmt.Sprintf("%s %d files retagged successfully", completedSymbol, m.successCount)))
		}
		lines = append(lines,
			errorStyle.Render(fmt.Sprintf("%s %d files failed", failedSymbol, len(m.failedFiles))),
			"",
		)
		for _, f := range m.failedFiles {
			lines = append(lines, errorStyle.Render(fmt.Sprintf("  - %s: %s", f.Filename, f.Error)))
		}
	}

	if m.errorMsg != "" {
		lines = append(lines, "", errorStyle.Render(m.errorMsg))
	}

	lines = append(lines, "", dimStyle.Render("[Enter] Close"))

	return strings.Join(lines, "\n")
}
