// Package jobbar displays long-running job progress at the bottom of the screen.
package jobbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// Height is the height of the job bar (content + borders).
const Height = 3

// Job represents a single long-running job.
type Job struct {
	ID      string
	Label   string
	Current int
	Total   int  // 0 if unknown
	Done    bool // true if job completed
}

// HasProgress returns true if the job has known progress (Total > 0).
func (j Job) HasProgress() bool {
	return j.Total > 0
}

// State holds the jobs to display.
type State struct {
	Jobs []Job
}

// HasActiveJobs returns true if there are any non-completed jobs.
func (s State) HasActiveJobs() bool {
	for _, j := range s.Jobs {
		if !j.Done {
			return true
		}
	}
	return false
}

func labelStyle() lipgloss.Style {
	return styles.T().S().Title
}

func progressStyle() lipgloss.Style {
	return styles.T().S().Muted
}

func barFilledStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(styles.T().Primary)
}

func barEmptyStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

// Render renders the job bar with the given width.
// Returns empty string if there are no active jobs.
func Render(state State, width int) string {
	if !state.HasActiveJobs() {
		return ""
	}

	// Find the first active job to display
	var activeJob *Job
	for i := range state.Jobs {
		if !state.Jobs[i].Done {
			activeJob = &state.Jobs[i]
			break
		}
	}

	if activeJob == nil {
		return ""
	}

	innerWidth := width - 2 // account for borders

	content := renderJobLine(*activeJob, innerWidth)

	return styles.PanelStyle(false).
		Width(innerWidth).
		Render(content)
}

// renderJobLine renders a single job as a one-line display with optional progress bar.
func renderJobLine(job Job, width int) string {
	if job.HasProgress() {
		return renderWithProgressBar(job, width)
	}
	return renderWithSpinner(job, width)
}

// renderWithProgressBar renders: "◦ Label  [━━━━────] 42/100"
func renderWithProgressBar(job Job, width int) string {
	// Spinner/indicator
	spinner := "◦"

	// Format: "◦ Label  [━━━━────] current/total"
	countStr := fmt.Sprintf("%d/%d", job.Current, job.Total)
	countWidth := lipgloss.Width(countStr)

	// Layout: spinner(1) + space(1) + label + space(2) + "[" + bar + "]" + space(1) + count
	spinnerWidth := 2 // "◦ "
	minBarWidth := 10
	brackets := 2 // "[]"
	spacing := 3  // spaces around bar
	fixedWidth := spinnerWidth + brackets + spacing + countWidth

	// Label gets remaining space (with some minimum)
	availableForLabel := max(width-fixedWidth-minBarWidth, 10)

	label := render.TruncateAndPad(job.Label, availableForLabel)

	// Bar gets what's left
	barWidth := max(width-availableForLabel-fixedWidth, minBarWidth)

	// Calculate fill
	ratio := float64(job.Current) / float64(job.Total)
	if ratio > 1 {
		ratio = 1
	}
	filled := int(float64(barWidth) * ratio)

	filledBar := barFilledStyle().Render(strings.Repeat("━", filled))
	emptyBar := barEmptyStyle().Render(strings.Repeat("─", barWidth-filled))

	var result strings.Builder
	result.WriteString(barFilledStyle().Render(spinner))
	result.WriteString(" ")
	result.WriteString(labelStyle().Render(label))
	result.WriteString("  [")
	result.WriteString(filledBar)
	result.WriteString(emptyBar)
	result.WriteString("] ")
	result.WriteString(progressStyle().Render(countStr))

	return result.String()
}

// renderWithSpinner renders: "⠋ Label                    123 files found"
func renderWithSpinner(job Job, width int) string {
	// Spinner character (static for now, could animate with tick)
	spinner := "◦"

	// Build the right side: count info
	var countInfo string
	if job.Current > 0 {
		countInfo = fmt.Sprintf("%d files found", job.Current)
	}

	// spinner(1) + space(1) + label + space(2) + countInfo
	spinnerWidth := 2 // "◦ "
	spacing := 2      // space before count
	countWidth := lipgloss.Width(countInfo)

	labelWidth := max(width-spinnerWidth-spacing-countWidth, 10)

	label := render.TruncateAndPad(job.Label, labelWidth)

	var result strings.Builder
	result.WriteString(barFilledStyle().Render(spinner))
	result.WriteString(" ")
	result.WriteString(labelStyle().Render(label))
	if countInfo != "" {
		result.WriteString("  ")
		result.WriteString(progressStyle().Render(countInfo))
	}

	return result.String()
}
