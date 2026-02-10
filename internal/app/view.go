// internal/app/view.go
package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/headerbar"
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

	// Render header bar
	var libSubMode headerbar.LibrarySubMode
	switch m.Navigation.LibrarySubMode() {
	case navctl.LibraryModeMiller:
		libSubMode = headerbar.LibraryModeMiller
	case navctl.LibraryModeAlbum:
		libSubMode = headerbar.LibraryModeAlbum
	case navctl.LibraryModeBrowser:
		libSubMode = headerbar.LibraryModeBrowser
	}
	header := headerbar.Render(string(m.Navigation.ViewMode()), m.Layout.Width(), m.HasSlskdConfig, libSubMode)

	// Render active navigator (special case for empty library and downloads)
	var navView string
	switch m.Navigation.ViewMode() {
	case navctl.ViewLibrary:
		if !m.HasLibrarySources {
			navView = m.renderEmptyLibrary()
		} else {
			navView = m.Navigation.RenderActiveNavigator()
		}
	case navctl.ViewFileBrowser, navctl.ViewPlaylists:
		navView = m.Navigation.RenderActiveNavigator()
	case navctl.ViewDownloads:
		navView = m.DownloadsView.View()
	}

	// Combine navigator and queue panel if visible
	var view string
	if m.Layout.IsQueueVisible() {
		if m.Layout.IsNarrowMode() {
			// Stack vertically in narrow mode
			view = navView + "\n" + m.Layout.RenderQueuePanel()
		} else {
			// Side by side in normal mode
			view = joinColumnsView(navView, m.Layout.RenderQueuePanel())
		}
	} else {
		view = navView
	}

	// Prepend header
	view = header + "\n" + view

	// Add player bar if playing
	if !m.PlaybackService.IsStopped() {
		view += "\n" + m.renderPlayerBar()
	}

	// Add job bar if there are active jobs
	if m.HasActiveJobs() {
		var jobs []jobbar.Job
		if m.LibraryScanJob != nil {
			jobs = append(jobs, *m.LibraryScanJob)
		}
		for _, job := range m.ExportJobs {
			jobs = append(jobs, *job.JobBar())
		}
		jobState := jobbar.State{Jobs: jobs}
		view += "\n" + jobbar.Render(jobState, m.Layout.Width())
	}

	// Add notification bar if there are notifications
	if len(m.Notifications) > 0 {
		view += "\n" + m.renderNotifications()
	}

	// Overlay search popup if active
	if m.Input.IsSearchActive() {
		searchView := m.Input.SearchView()
		view = popup.Compose(view, searchView, m.Layout.Width(), m.Layout.Height())
	}

	// Overlay all popups
	view = m.Popups.RenderOverlay(view)

	// Ensure view is exactly terminal height (pad or truncate if needed)
	view = enforceHeight(view, m.Layout.Height())

	// Prepend album art transmission if pending (sent once per track)
	if m.albumArtPendingTransmit != "" {
		view = m.albumArtPendingTransmit + view
	}

	// Append album art placement command (Kitty graphics protocol)
	view += m.getAlbumArtPlacement()

	return view
}

// getAlbumArtPlacement returns the Kitty graphics placement command for album art.
// Returns empty string if no album art should be displayed.
func (m Model) getAlbumArtPlacement() string {
	if m.AlbumArt == nil || !m.AlbumArt.HasImage() {
		return ""
	}

	// Only show in expanded mode
	if m.Layout.PlayerDisplayMode() != playerbar.ModeExpanded {
		return ""
	}

	// Calculate position: player bar row + 1 (for top border)
	// Column: left border (1) + horizontal padding (2) + 1
	playerRow := m.PlayerBarRow()
	if playerRow == 0 {
		return ""
	}

	// Image row is inside the player bar: just after top border
	// expandedBarStyle has Padding(0, 2) - no vertical padding, just horizontal
	imageRow := playerRow + 1 // +1 for top border only
	imageCol := 4             // left border (1) + horizontal padding (2) + 1

	return m.AlbumArt.GetPlacementCmd(imageRow, imageCol)
}

// enforceHeight ensures the view has exactly the specified number of lines.
func enforceHeight(view string, targetHeight int) string {
	lines := splitLines(view)
	currentHeight := len(lines)

	if currentHeight == targetHeight {
		return view
	}

	if currentHeight < targetHeight {
		// Pad with empty lines
		for i := currentHeight; i < targetHeight; i++ {
			lines = append(lines, "")
		}
	} else {
		// Truncate (shouldn't normally happen)
		lines = lines[:targetHeight]
	}

	return strings.Join(lines, "\n")
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

	t := styles.T()
	titleStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	waveStyle := lipgloss.NewStyle().
		Foreground(t.Secondary)

	statusStyle := t.S().Muted.Italic(true)

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

// renderPlayerBar renders the player bar with radio state.
func (m Model) renderPlayerBar() string {
	state := playerbar.NewState(m.PlaybackService.Player(), m.Layout.PlayerDisplayMode())
	state.RadioEnabled = m.PlaybackService.RepeatMode() == playback.RepeatRadio

	// Set up album art placeholder for expanded mode
	if state.DisplayMode == playerbar.ModeExpanded && state.TrackPath != "" && m.AlbumArt != nil {
		state.HasAlbumArt = m.AlbumArt.HasImage()
		if state.HasAlbumArt {
			state.AlbumArtPlaceholder = m.AlbumArt.GetPlaceholder()
		}
	}

	return playerbar.Render(state, m.Layout.Width())
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

Press  f p  to open the library sources manager
and add a music folder to get started.`

	t := styles.T()
	messageStyle := t.S().Muted

	hintStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
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
	focused := m.Navigation.IsNavigatorFocused()
	return styles.PanelStyle(focused).Width(innerWidth).Render(content)
}

// renderNotifications renders all notification messages.
func (m Model) renderNotifications() string {
	if len(m.Notifications) == 0 {
		return ""
	}

	t := styles.T()
	innerWidth := m.Layout.Width() - 2 // Account for borders

	// Style: checkmark + message
	checkStyle := lipgloss.NewStyle().Foreground(t.Primary)
	msgStyle := lipgloss.NewStyle().Foreground(t.FgBase)

	lines := make([]string, 0, len(m.Notifications))
	for _, n := range m.Notifications {
		line := checkStyle.Render("✓") + " " + msgStyle.Render(n.Message)
		line = render.TruncateAndPad(line, innerWidth)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")
	return styles.PanelStyle(false).Width(innerWidth).Render(content)
}
