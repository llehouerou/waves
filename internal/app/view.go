// internal/app/view.go
package app

import (
	"strings"

	"github.com/llehouerou/waves/internal/player"
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

	// Show library scan progress in header area if scanning
	if m.LibraryScanMsg != "" {
		lines := splitLines(navView)
		if len(lines) > 0 {
			lines[0] = m.LibraryScanMsg
			navView = joinLines(lines)
		}
	}

	// Combine navigator and queue panel if visible
	var view string
	if m.QueueVisible {
		view = joinColumnsView(navView, m.QueuePanel.View())
	} else {
		view = navView
	}

	if m.Player.State() != player.Stopped {
		barState := playerbar.NewState(m.Player, m.PlayerDisplayMode)
		view += "\n" + playerbar.Render(barState, m.Width)
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

// joinLines joins lines into a single string.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(lines[0])
	for i := 1; i < len(lines); i++ {
		sb.WriteByte('\n')
		sb.WriteString(lines[i])
	}
	return sb.String()
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
