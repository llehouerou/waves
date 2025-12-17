package download

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

const (
	filterOn  = "on"
	filterOff = "off"
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

	// Status/error messages (reserve 2 lines always for consistent layout)
	if m.errorMsg != "" {
		errText := "Error: " + m.errorMsg
		b.WriteString(errorStyle.Width(m.width - 4).Render(errText))
	}
	b.WriteString("\n")
	if m.statusMsg != "" {
		b.WriteString(statusStyle.Render(m.statusMsg))
	}
	b.WriteString("\n")

	// Current step content
	switch m.state {
	case StateSearch, StateArtistSearching:
		b.WriteString(m.renderSearchSection())
	case StateArtistResults:
		b.WriteString(m.renderArtistResults())
	case StateReleaseGroupLoading, StateReleaseGroupResults:
		b.WriteString(m.renderReleaseGroupResults())
	case StateReleaseLoading, StateReleaseResults, StateReleaseDetailsLoading:
		b.WriteString(m.renderReleaseResults())
	case StateSlskdSearching, StateSlskdResults, StateDownloading:
		b.WriteString(m.renderSlskdResults())
	}

	// Help section at bottom
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	// Pad output to fixed height to prevent layout jitter
	return m.padToHeight(b.String())
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
	case StateReleaseGroupLoading, StateReleaseGroupResults, StateReleaseLoading, StateReleaseResults, StateReleaseDetailsLoading:
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
		albumsFilter := filterOn
		if !m.albumsOnly {
			albumsFilter = filterOff
		}
		help = fmt.Sprintf("↑/↓: Navigate | Enter: Select | a: Albums [%s] | Backspace: Back | Esc: Close", albumsFilter)
	case StateReleaseLoading:
		help = "Loading track info... | Esc: Close"
	case StateReleaseResults:
		dedupFilter := filterOn
		if !m.deduplicateRelease {
			dedupFilter = filterOff
		}
		help = fmt.Sprintf("↑/↓: Navigate | Enter: Select | d: Dedup [%s] | Backspace: Back | Esc: Close", dedupFilter)
	case StateReleaseDetailsLoading:
		help = "Loading release details... | Esc: Close"
	case StateSlskdSearching:
		help = "Searching slskd... | Esc: Close"
	case StateSlskdResults:
		help = "↑/↓: Navigate | Enter: Download | f/s/t: Filters | Backspace: Back | Esc: Close"
	case StateDownloading:
		help = "Backspace: Back | Esc: Close"
	}
	return dimStyle.Render(help)
}

// formatSize formats a file size in human-readable form.
// Uses binary calculation (1024) with SI notation (KB, MB, GB).
func formatSize(bytes int64) string {
	if bytes < 0 {
		bytes = 0
	}
	s := humanize.IBytes(uint64(bytes)) //nolint:gosec // bytes is guaranteed non-negative above
	// Convert IEC notation to SI: GiB→GB, MiB→MB, KiB→KB
	s = strings.ReplaceAll(s, "iB", "B")
	return s
}

// padToHeight pads the content to a fixed height to prevent layout jitter.
func (m *Model) padToHeight(content string) string {
	lines := strings.Split(content, "\n")
	currentHeight := len(lines)

	// Target height is the available content height (minus padding/borders handled by popup)
	targetHeight := m.height - 4 // Leave some margin

	if currentHeight >= targetHeight {
		// Already at or exceeding target, return as-is
		return content
	}

	// Pad with empty lines
	padding := targetHeight - currentHeight
	for range padding {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
