package download

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/musicbrainz"
)

var (
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	typeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	// Step indicator styles
	stepActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	stepCompletedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46"))

	stepPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	stepValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))
)

// View renders the download view.
func (m *Model) View() string {
	var b strings.Builder

	// Step indicator
	b.WriteString(m.renderStepIndicator())
	b.WriteString("\n\n")

	// Status/error messages
	if m.errorMsg != "" {
		errText := "Error: " + m.errorMsg
		b.WriteString(errorStyle.Width(m.width - 4).Render(errText))
		b.WriteString("\n\n")
	}
	if m.statusMsg != "" {
		b.WriteString(statusStyle.Render(m.statusMsg))
		b.WriteString("\n\n")
	}

	// Current step content
	switch m.state {
	case StateSearch, StateArtistSearching:
		b.WriteString(m.renderSearchSection())
	case StateArtistResults:
		b.WriteString(m.renderArtistResults())
	case StateReleaseGroupLoading, StateReleaseGroupResults:
		b.WriteString(m.renderReleaseGroupResults())
	case StateReleaseLoading, StateReleaseResults:
		b.WriteString(m.renderReleaseResults())
	case StateSlskdSearching, StateSlskdResults, StateDownloading:
		b.WriteString(m.renderSlskdResults())
	}

	// Help section at bottom
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return b.String()
}

// renderStepIndicator renders the step progress indicator.
func (m *Model) renderStepIndicator() string {
	// Determine current step (1-3)
	currentStep := m.getCurrentStep()

	var b strings.Builder

	// Step 1: Artist
	step1Style := m.getStepStyle(1, currentStep)
	b.WriteString(step1Style.Render("① Artist"))
	if m.selectedArtist != nil {
		b.WriteString(" ")
		b.WriteString(stepValueStyle.Render("✓ " + truncateName(m.selectedArtist.Name, 20)))
	}

	b.WriteString(dimStyle.Render("  →  "))

	// Step 2: Release
	step2Style := m.getStepStyle(2, currentStep)
	b.WriteString(step2Style.Render("② Release"))
	if m.selectedReleaseGroup != nil {
		info := m.selectedReleaseGroup.Title
		if m.selectedReleaseGroup.FirstRelease != "" {
			year := m.selectedReleaseGroup.FirstRelease
			if len(year) > 4 {
				year = year[:4]
			}
			info += " (" + year + ")"
		}
		if m.expectedTracks > 0 {
			info += fmt.Sprintf(" [%d tracks]", m.expectedTracks)
		}
		b.WriteString(" ")
		b.WriteString(stepValueStyle.Render("✓ " + truncateName(info, 40)))
	}

	b.WriteString(dimStyle.Render("  →  "))

	// Step 3: Source
	step3Style := m.getStepStyle(3, currentStep)
	b.WriteString(step3Style.Render("③ Source"))

	return b.String()
}

// getCurrentStep returns the current step number (1-3).
func (m *Model) getCurrentStep() int {
	switch m.state {
	case StateSearch, StateArtistSearching, StateArtistResults:
		return 1
	case StateReleaseGroupLoading, StateReleaseGroupResults, StateReleaseLoading, StateReleaseResults:
		return 2
	case StateSlskdSearching, StateSlskdResults, StateDownloading:
		return 3
	}
	return 1
}

// getStepStyle returns the appropriate style for a step.
func (m *Model) getStepStyle(step, currentStep int) lipgloss.Style {
	if step == currentStep {
		return stepActiveStyle
	}
	if step < currentStep {
		return stepCompletedStyle
	}
	return stepPendingStyle
}

// truncateName truncates a name to fit within maxLen (from the end).
func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	if maxLen > 3 {
		return name[:maxLen-3] + "..."
	}
	return name[:maxLen]
}

// truncateDirectory truncates a directory path to fit within maxLen.
// Truncates from the beginning to keep the most relevant part (album name).
func truncateDirectory(dir string, maxLen int) string {
	if len(dir) <= maxLen {
		return dir
	}
	if maxLen > 3 {
		return "..." + dir[len(dir)-(maxLen-3):]
	}
	return dir[len(dir)-maxLen:]
}

// renderSearchSection renders the search input.
func (m *Model) renderSearchSection() string {
	var b strings.Builder
	b.WriteString(dimStyle.Render("Search for an artist:"))
	b.WriteString("\n")
	b.WriteString(m.searchInput.View())
	return b.String()
}

// renderArtistResults renders the artist search results.
func (m *Model) renderArtistResults() string {
	if len(m.artistResults) == 0 {
		return dimStyle.Render("No artists found")
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render("Select an artist:"))
	b.WriteString("\n\n")

	maxVisible := max(m.height-12, 5)
	start, end := m.calculateVisibleRange(m.artistCursor, len(m.artistResults), maxVisible)

	for i := start; i < end; i++ {
		a := &m.artistResults[i]
		line := m.formatArtist(a)

		if i == m.artistCursor {
			b.WriteString(cursorStyle.Render("> "))
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString("  ")
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// formatArtist formats a single artist.
func (m *Model) formatArtist(a *musicbrainz.Artist) string {
	parts := []string{a.Name}

	// Add disambiguation if present (e.g., "British rock band")
	if a.Disambiguation != "" {
		parts = append(parts, dimStyle.Render("("+a.Disambiguation+")"))
	}

	// Add type (Person, Group, etc.)
	if a.Type != "" {
		parts = append(parts, typeStyle.Render("["+a.Type+"]"))
	}

	// Add country
	if a.Country != "" {
		parts = append(parts, dimStyle.Render("["+a.Country+"]"))
	}

	// Add life span (e.g., "1965-" or "1965-2020")
	if a.BeginYear != "" {
		lifeSpan := a.BeginYear + "-"
		if a.EndYear != "" {
			lifeSpan += a.EndYear
		}
		parts = append(parts, dimStyle.Render(lifeSpan))
	}

	return strings.Join(parts, " ")
}

// renderReleaseGroupResults renders the release groups grouped by type.
func (m *Model) renderReleaseGroupResults() string {
	if len(m.releaseGroups) == 0 {
		return dimStyle.Render("No releases found")
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render("Select a release:"))
	b.WriteString("\n\n")

	maxVisible := max(m.height-12, 5)
	start, end := m.calculateVisibleRange(m.releaseGroupCursor, len(m.releaseGroups), maxVisible)

	for i := start; i < end; i++ {
		rg := &m.releaseGroups[i]
		line := m.formatReleaseGroup(rg)

		if i == m.releaseGroupCursor {
			b.WriteString(cursorStyle.Render("> "))
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString("  ")
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// formatReleaseGroup formats a single release group.
func (m *Model) formatReleaseGroup(rg *musicbrainz.ReleaseGroup) string {
	parts := []string{rg.Title}

	if rg.FirstRelease != "" {
		year := rg.FirstRelease
		if len(year) > 4 {
			year = year[:4]
		}
		parts = append(parts, fmt.Sprintf("(%s)", year))
	}

	if rg.PrimaryType != "" {
		parts = append(parts, typeStyle.Render(fmt.Sprintf("[%s]", rg.PrimaryType)))
	}

	if len(rg.SecondaryTypes) > 0 {
		parts = append(parts, dimStyle.Render("+"+strings.Join(rg.SecondaryTypes, "+")))
	}

	return strings.Join(parts, " ")
}

// renderReleaseResults renders the releases for track count selection.
func (m *Model) renderReleaseResults() string {
	if len(m.releases) == 0 {
		return dimStyle.Render("No releases found")
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render("Select a release (different track counts detected):"))
	b.WriteString("\n\n")

	maxVisible := max(m.height-12, 5)
	start, end := m.calculateVisibleRange(m.releaseCursor, len(m.releases), maxVisible)

	for i := start; i < end; i++ {
		r := &m.releases[i]
		line := m.formatRelease(r)

		if i == m.releaseCursor {
			b.WriteString(cursorStyle.Render("> "))
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString("  ")
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// formatRelease formats a single release for display.
func (m *Model) formatRelease(r *musicbrainz.Release) string {
	parts := []string{r.Title}

	// Track count (most important)
	parts = append(parts, typeStyle.Render(fmt.Sprintf("[%d tracks]", r.TrackCount)))

	// Date
	if r.Date != "" {
		year := r.Date
		if len(year) > 4 {
			year = year[:4]
		}
		parts = append(parts, fmt.Sprintf("(%s)", year))
	}

	// Country
	if r.Country != "" {
		parts = append(parts, dimStyle.Render("["+r.Country+"]"))
	}

	// Formats (CD, Vinyl, Digital, etc.)
	if r.Formats != "" {
		parts = append(parts, dimStyle.Render(r.Formats))
	}

	return strings.Join(parts, " ")
}

// renderSlskdResults renders the slskd search results as a table.
func (m *Model) renderSlskdResults() string {
	if len(m.slskdResults) == 0 {
		// Don't show "no sources" while still searching
		if m.state == StateSlskdSearching {
			return ""
		}
		return dimStyle.Render("No sources found")
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render("Select a download source:"))
	b.WriteString("\n\n")

	// Column widths - fixed columns plus dynamic directory
	const (
		colUser     = 18
		colFormat   = 8
		colBitRate  = 6
		colFiles    = 5
		colSize     = 9
		colSpeed    = 10
		fixedWidth  = colUser + colFormat + colBitRate + colFiles + colSize + colSpeed + 12 // spacing + cursor
		minDirWidth = 20
		maxDirWidth = 50
	)
	// Directory gets remaining space, clamped to min/max
	colDir := min(max(m.width-fixedWidth, minDirWidth), maxDirWidth)

	// Header
	header := fmt.Sprintf("  %-*s %-*s %-*s %*s %*s %*s %*s",
		colUser, "User",
		colDir, "Directory",
		colFormat, "Format",
		colBitRate, "kbps",
		colFiles, "Files",
		colSize, "Size",
		colSpeed, "Speed")
	b.WriteString(dimStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", colUser+colDir+colFormat+colBitRate+colFiles+colSize+colSpeed+9)))
	b.WriteString("\n")

	maxVisible := max(m.height-14, 5)
	start, end := m.calculateVisibleRange(m.slskdCursor, len(m.slskdResults), maxVisible)

	for i := start; i < end; i++ {
		r := &m.slskdResults[i]

		// Format each column
		user := truncateName(r.Username, colUser)
		dir := truncateDirectory(r.Directory, colDir)
		format := r.Format
		bitrate := formatBitRate(r.BitRate)
		files := strconv.Itoa(r.FileCount)
		size := formatSize(r.TotalSize)
		speed := formatSpeed(r.UploadSpeed)

		// Build row
		row := fmt.Sprintf("%-*s %-*s %-*s %*s %*s %*s %*s",
			colUser, user,
			colDir, dir,
			colFormat, format,
			colBitRate, bitrate,
			colFiles, files,
			colSize, size,
			colSpeed, speed)

		if i == m.slskdCursor {
			b.WriteString(cursorStyle.Render("> "))
			b.WriteString(selectedStyle.Render(row))
		} else {
			b.WriteString("  ")
			b.WriteString(row)
		}
		b.WriteString("\n")
	}

	// Show filter controls
	b.WriteString("\n")
	b.WriteString(m.renderFilterControls())

	// Show filter stats
	b.WriteString("\n")
	b.WriteString(m.renderFilterStats())

	return b.String()
}

// renderFilterControls renders the current filter settings.
func (m *Model) renderFilterControls() string {
	var parts []string

	// Format filter
	var formatLabel string
	switch m.formatFilter {
	case FormatBoth:
		formatLabel = "Both"
	case FormatLossless:
		formatLabel = "Lossless"
	case FormatLossy:
		formatLabel = "Lossy"
	}
	parts = append(parts, "[f] Format: "+formatLabel)

	// No slot filter
	slotLabel := "off"
	if m.filterNoSlot {
		slotLabel = "on"
	}
	parts = append(parts, "[s] No slot: "+slotLabel)

	// Track count filter
	trackLabel := "off"
	if m.filterTrackCount {
		trackLabel = "on"
	}
	parts = append(parts, "[t] Track count: "+trackLabel)

	return dimStyle.Render(strings.Join(parts, "  |  "))
}

// renderFilterStats renders the filter statistics.
func (m *Model) renderFilterStats() string {
	s := m.filterStats

	var parts []string

	// Show what was filtered out
	if s.NoFreeSlot > 0 {
		parts = append(parts, fmt.Sprintf("no slot: %d", s.NoFreeSlot))
	}
	if s.NoAudioFiles > 0 {
		parts = append(parts, fmt.Sprintf("no audio: %d", s.NoAudioFiles))
	}
	if s.InsufficientTracks > 0 {
		parts = append(parts, fmt.Sprintf("<%d tracks: %d", s.ExpectedTracks, s.InsufficientTracks))
	}

	if len(parts) == 0 {
		return ""
	}

	result := "Filtered out: " + strings.Join(parts, ", ")
	return dimStyle.Render(result)
}

// formatBitRate formats bitrate for display.
// Returns "-" if bitrate is 0 (typically lossless formats).
func formatBitRate(kbps int) string {
	if kbps == 0 {
		return "-"
	}
	return strconv.Itoa(kbps)
}

// formatSpeed formats upload speed in human-readable form.
func formatSpeed(bytesPerSec int) string {
	const (
		kb = 1024
		mb = kb * 1024
	)

	switch {
	case bytesPerSec >= mb:
		return fmt.Sprintf("%.1f MB/s", float64(bytesPerSec)/float64(mb))
	case bytesPerSec >= kb:
		return fmt.Sprintf("%.0f KB/s", float64(bytesPerSec)/float64(kb))
	default:
		return fmt.Sprintf("%d B/s", bytesPerSec)
	}
}

// renderHelp renders context-sensitive help.
func (m *Model) renderHelp() string {
	var help string
	switch m.state {
	case StateSearch:
		help = "Enter: Search | Esc: Close"
	case StateArtistSearching:
		help = "Searching artists... | Esc: Close"
	case StateArtistResults:
		help = "↑/↓: Navigate | Enter: Select | Backspace: Back | Esc: Close"
	case StateReleaseGroupLoading:
		help = "Loading releases... | Esc: Close"
	case StateReleaseGroupResults:
		help = "↑/↓: Navigate | Enter: Select | Backspace: Back | Esc: Close"
	case StateReleaseLoading:
		help = "Loading track info... | Esc: Close"
	case StateReleaseResults:
		help = "↑/↓: Navigate | Enter: Select track count | Backspace: Back | Esc: Close"
	case StateSlskdSearching:
		help = "Searching slskd... | Esc: Close"
	case StateSlskdResults:
		help = "↑/↓: Navigate | Enter: Download | f/s/t: Filters | Backspace: Back | Esc: Close"
	case StateDownloading:
		help = "Backspace: Back | Esc: Close"
	}
	return dimStyle.Render(help)
}

// calculateVisibleRange calculates the start and end indices for visible items.
func (m *Model) calculateVisibleRange(cursor, total, maxVisible int) (start, end int) {
	if total <= maxVisible {
		return 0, total
	}

	half := maxVisible / 2
	start = max(cursor-half, 0)
	end = start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
	}

	return start, end
}

// formatSize formats a file size in human-readable form.
func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
