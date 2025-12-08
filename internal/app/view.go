// internal/app/view.go
package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/jobbar"
	"github.com/llehouerou/waves/internal/ui/playerbar"
	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// View renders the application UI.
func (m Model) View() string {
	// During loading, show appropriate screen
	switch m.loadingState {
	case loadingWaiting:
		// Brief blank screen while waiting to see if we need to show loading
		return ""
	case loadingShowing:
		return m.renderLoading()
	case loadingDone:
		// Continue to normal rendering below
	}

	// Render active navigator
	var navView string
	switch m.ViewMode {
	case ViewFileBrowser:
		navView = m.FileNavigator.View()
	case ViewPlaylists:
		navView = m.PlaylistNavigator.View()
	case ViewLibrary:
		if m.HasLibrarySources {
			navView = m.LibraryNavigator.View()
		} else {
			navView = m.renderEmptyLibrary()
		}
	}

	// Combine navigator and queue panel if visible
	var view string
	if m.Layout.IsQueueVisible() {
		view = joinColumnsView(navView, m.Layout.QueuePanel().View())
	} else {
		view = navView
	}

	// Add player bar if playing
	if m.Player.State() != player.Stopped {
		barState := playerbar.NewState(m.Player, m.PlayerDisplayMode)
		view += "\n" + playerbar.Render(barState, m.Layout.Width())
	}

	// Add job bar if there are active jobs
	if m.HasActiveJobs() {
		jobState := jobbar.State{
			Jobs: []jobbar.Job{*m.LibraryScanJob},
		}
		view += "\n" + jobbar.Render(jobState, m.Layout.Width())
	}

	// Overlay search popup if active
	if m.Input.IsSearchActive() {
		searchView := m.Input.SearchView()
		view = popup.Compose(view, searchView, m.Layout.Width(), m.Layout.Height())
	}

	// Overlay all popups
	view = m.Popups.RenderOverlay(view)

	return view
}

func (m Model) renderLoading() string {
	// Can't render before we know terminal size
	if m.Layout.Width() == 0 || m.Layout.Height() == 0 {
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

	buildWaveLine := func(offset int) string {
		var sb strings.Builder
		for i := range waveWidth {
			charIdx := (i + offset) % len(waveChars)
			sb.WriteRune(waveChars[charIdx])
		}
		return sb.String()
	}
	waveLine0 := buildWaveLine(m.LoadingFrame)
	waveLine1 := buildWaveLine(m.LoadingFrame + 2)

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
		waveStyle.Render(wavePad+waveLine0) + "\n" +
		waveStyle.Render(wavePad+waveLine1) + "\n\n" +
		statusStyle.Render(statusPad+m.LoadingStatus)

	return popup.Center(content, m.Layout.Width(), m.Layout.Height())
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

// renderEmptyLibrary renders a helpful message when no library sources are configured.
func (m Model) renderEmptyLibrary() string {
	// Use same dimensions as navigator panel
	innerWidth := m.Layout.NavigatorWidth() - ui.BorderHeight
	listHeight := m.NavigatorHeight() - ui.PanelOverhead

	// Header and separator (like navigator)
	header := render.TruncateAndPad("Library", innerWidth)
	separator := render.Separator(innerWidth)

	// Build the message
	message := `No library sources configured.

Press  g p  to open the library sources manager
and add a music folder to get started.`

	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	// Style the key binding
	msgLines := strings.Split(message, "\n")
	styledLines := make([]string, len(msgLines))
	for i, line := range msgLines {
		line = strings.Replace(line, "g p", hintStyle.Render("g p"), 1)
		styledLines[i] = messageStyle.Render(line)
	}

	// Build content area that fills the full height with centered message
	contentLines := make([]string, listHeight)
	msgHeight := len(styledLines)
	startLine := max(0, (listHeight-msgHeight)/2)

	for i := range listHeight {
		msgIdx := i - startLine
		if msgIdx >= 0 && msgIdx < msgHeight {
			// Center the message line horizontally
			line := styledLines[msgIdx]
			lineWidth := lipgloss.Width(line)
			padLeft := max(0, (innerWidth-lineWidth)/2)
			contentLines[i] = strings.Repeat(" ", padLeft) + line
		} else {
			contentLines[i] = ""
		}
		// Pad to full width
		contentLines[i] = render.Pad(contentLines[i], innerWidth)
	}

	content := header + "\n" + separator + "\n" + strings.Join(contentLines, "\n")

	// Wrap with panel style (focused since navigator has focus in library view)
	focused := m.Focus == FocusNavigator
	return styles.PanelStyle(focused).Width(innerWidth).Render(content)
}
