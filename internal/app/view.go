// internal/app/view.go
package app

import (
	"strings"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/playerbar"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// View renders the application UI.
func (m Model) View() string {
	// Render active navigator
	var navView string
	if m.ViewMode == ViewFileBrowser {
		navView = m.FileNavigator.View()
	} else {
		navView = m.LibraryNavigator.View()
	}

	// Combine navigator and queue panel if visible
	var view string
	if m.QueueVisible {
		view = joinColumnsView(navView, m.QueuePanel.View())
	} else {
		view = navView
	}

	// Add player bar if playing
	if m.Player.State() != player.Stopped {
		barState := playerbar.NewState(m.Player, m.PlayerDisplayMode)
		view += "\n" + playerbar.Render(barState, m.Width)
	}

	// Add job bar if there are active jobs
	if m.HasActiveJobs() {
		jobState := jobbar.State{
			Jobs: []jobbar.Job{*m.LibraryScanJob},
		}
		view += "\n" + jobbar.Render(jobState, m.Width)
	}

	// Overlay search popup if active
	if m.SearchMode {
		searchView := m.Search.View()
		view = popup.Compose(view, searchView, m.Width, m.Height)
	}

	// Overlay error popup if present
	if m.ErrorMsg != "" {
		errorView := m.renderError()
		view = popup.Compose(view, errorView, m.Width, m.Height)
	}

	return view
}

func (m Model) renderError() string {
	p := popup.New()
	p.Title = "Error"
	p.Content = m.ErrorMsg
	p.Footer = "Press any key to dismiss"
	return p.Render(m.Width, m.Height)
}

// splitLines splits a string into lines without using strings.Split.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// joinColumnsView joins two column views side by side.
func joinColumnsView(left, right string) string {
	leftLines := splitLines(left)
	rightLines := splitLines(right)

	lineCount := max(len(leftLines), len(rightLines))

	var sb strings.Builder
	for i := range lineCount {
		if i < len(leftLines) {
			sb.WriteString(leftLines[i])
		}
		if i < len(rightLines) {
			sb.WriteString(rightLines[i])
		}
		if i < lineCount-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
