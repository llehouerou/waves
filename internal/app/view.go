// internal/app/view.go
package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/playerbar"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// View renders the application UI.
func (m Model) View() string {
	// Show loading screen during initialization
	if m.Loading {
		return m.renderLoading()
	}

	// Render active navigator
	var navView string
	switch m.ViewMode {
	case ViewFileBrowser:
		navView = m.FileNavigator.View()
	case ViewPlaylists:
		navView = m.PlaylistNavigator.View()
	case ViewLibrary:
		if m.hasLibrarySources() {
			navView = m.LibraryNavigator.View()
		} else {
			navView = m.renderEmptyLibrary()
		}
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
	if m.SearchMode || m.AddToPlaylistMode {
		searchView := m.Search.View()
		view = popup.Compose(view, searchView, m.Width, m.Height)
	}

	// Overlay text input popup if active
	if m.InputMode != InputNone {
		inputView := m.TextInput.View()
		view = popup.Compose(view, inputView, m.Width, m.Height)
	}

	// Overlay confirmation popup if active
	if m.Confirm.Active() {
		confirmView := m.Confirm.View()
		view = popup.Compose(view, confirmView, m.Width, m.Height)
	}

	// Overlay library sources popup if active
	if m.ShowLibrarySourcesPopup {
		sourcesView := m.LibrarySourcesPopup.View()
		view = popup.Compose(view, sourcesView, m.Width, m.Height)
	}

	// Overlay error popup if present
	if m.ErrorMsg != "" {
		errorView := m.renderError()
		view = popup.Compose(view, errorView, m.Width, m.Height)
	}

	// Overlay scan report popup if present
	if m.ScanReportPopup != nil {
		reportView := m.ScanReportPopup.Render()
		view = popup.Compose(view, reportView, m.Width, m.Height)
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

func (m Model) renderLoading() string {
	// Can't render before we know terminal size
	if m.Width == 0 || m.Height == 0 {
		return ""
	}

	logo := `
     ██╗    ██╗ █████╗ ██╗   ██╗███████╗███████╗
     ██║    ██║██╔══██╗██║   ██║██╔════╝██╔════╝
     ██║ █╗ ██║███████║██║   ██║█████╗  ███████╗
     ██║███╗██║██╔══██║╚██╗ ██╔╝██╔══╝  ╚════██║
     ╚███╔███╔╝██║  ██║ ╚████╔╝ ███████╗███████║
      ╚══╝╚══╝ ╚═╝  ╚═╝  ╚═══╝  ╚══════╝╚══════╝
`

	// Animated wave - 2 lines, centered under logo
	waveChars := []rune{'~', '∿', '≈', '∼', '≈', '∿'}
	waveWidth := 36

	var waveLines [2]string
	for line := range 2 {
		var sb strings.Builder
		offset := m.LoadingFrame + line*2
		for i := range waveWidth {
			charIdx := (i + offset) % len(waveChars)
			sb.WriteRune(waveChars[charIdx])
		}
		waveLines[line] = sb.String()
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	waveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("31"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Italic(true)

	// Center everything
	logoWidth := 53 // Width of the logo
	wavePad := strings.Repeat(" ", (logoWidth-waveWidth)/2)
	statusPad := strings.Repeat(" ", (logoWidth-len(m.LoadingStatus))/2)

	content := titleStyle.Render(logo) + "\n" +
		waveStyle.Render(wavePad+waveLines[0]) + "\n" +
		waveStyle.Render(wavePad+waveLines[1]) + "\n\n" +
		statusStyle.Render(statusPad+m.LoadingStatus)

	return popup.Center(content, m.Width, m.Height)
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

// hasLibrarySources returns true if at least one library source is configured.
func (m Model) hasLibrarySources() bool {
	sources, err := m.Library.Sources()
	if err != nil {
		return false
	}
	return len(sources) > 0
}

// renderEmptyLibrary renders a helpful message when no library sources are configured.
func (m Model) renderEmptyLibrary() string {
	message := `No library sources configured.

Press  g p  to open the library sources manager
and add a music folder to get started.`

	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	// Style the key binding
	lines := strings.Split(message, "\n")
	styledLines := make([]string, len(lines))
	for i, line := range lines {
		// Replace "g p" with styled version (no-op if not present)
		line = strings.Replace(line, "g p", hintStyle.Render("g p"), 1)
		styledLines[i] = messageStyle.Render(line)
	}

	content := strings.Join(styledLines, "\n")

	// Center in available space
	return popup.Center(content, m.NavigatorWidth(), m.NavigatorHeight())
}
