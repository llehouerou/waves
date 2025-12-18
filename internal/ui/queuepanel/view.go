package queuepanel

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// View renders the queue panel.
func (m Model) View() string {
	if m.Width() == 0 || m.Height() == 0 {
		return ""
	}

	innerWidth := m.Width() - ui.BorderHeight // border padding
	listHeight := m.listHeight()

	// Header with mode icons on the right
	header := m.renderHeader(innerWidth)

	// Separator
	separator := render.Separator(innerWidth)

	// Track list
	trackList := m.renderTrackList(innerWidth, listHeight)

	content := header + "\n" + separator + "\n" + trackList

	return styles.PanelStyle(m.IsFocused()).
		Width(innerWidth).
		Render(content)
}

// renderHeader renders the queue header with track count and mode icons.
func (m Model) renderHeader(innerWidth int) string {
	var headerLeftText string
	var headerStyle lipgloss.Style
	if len(m.selected) > 0 {
		headerLeftText = fmt.Sprintf("Queue [%d selected]", len(m.selected))
		headerStyle = multiSelectHeaderStyle
	} else {
		currentIdx := m.queue.CurrentIndex() + 1
		if currentIdx < 1 {
			currentIdx = 0
		}
		headerLeftText = fmt.Sprintf("Queue (%d/%d)", currentIdx, m.queue.Len())
		headerStyle = defaultHeaderStyle
	}

	// Mode icons on the right
	modeIcons, modeIconsWidth := m.renderModeIcons()

	// Calculate available width for header text (truncate/pad raw text, then style)
	headerLeftWidth := innerWidth - modeIconsWidth
	headerLeftText = render.TruncateAndPad(headerLeftText, headerLeftWidth)

	return headerStyle.Render(headerLeftText) + modeIcons
}

// renderModeIcons returns the styled mode icons and their display width.
func (m Model) renderModeIcons() (styled string, width int) {
	var parts []string

	if m.queue.Shuffle() {
		parts = append(parts, icons.Shuffle())
	}

	switch m.queue.RepeatMode() {
	case playlist.RepeatOff:
		// No icon for repeat off
	case playlist.RepeatAll:
		parts = append(parts, icons.RepeatAll())
	case playlist.RepeatOne:
		parts = append(parts, icons.RepeatOne())
	}

	if len(parts) == 0 {
		return "", 0
	}

	// Join with double space for better separation
	raw := strings.Join(parts, "  ")
	// Icons are 1 cell wide each, plus 2 spaces between, plus 1 space padding from border
	width = len(parts) + (len(parts)-1)*2 + 1
	styled = modeIconStyle.Render(raw) + " "
	return styled, width
}

// renderTrackList renders the list of tracks.
func (m Model) renderTrackList(innerWidth, listHeight int) string {
	tracks := m.queue.Tracks()
	playingIdx := m.queue.CurrentIndex()

	lines := make([]string, 0, listHeight)
	for i := range listHeight {
		idx := i + m.cursor.Offset()
		if idx >= len(tracks) {
			lines = append(lines, render.EmptyLine(innerWidth))
			continue
		}

		track := tracks[idx]
		line := m.renderTrackLine(track, idx, playingIdx, innerWidth)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderTrackLine renders a single track line with prefix, title, artist, and suffix.
func (m Model) renderTrackLine(track playlist.Track, idx, playingIdx, width int) string {
	// Prefix: "▶ " for playing, "  " otherwise
	prefix := "  "
	if idx == playingIdx {
		prefix = playingSymbol + " "
	}

	// Always reserve space for selection marker
	suffixWidth := 2 // " ●"
	suffix := "  "
	if m.selected[idx] {
		suffix = " " + selectedSymbol
	}

	// Calculate available width for content
	prefixWidth := 2
	contentWidth := width - prefixWidth - suffixWidth

	// Two-column layout: title on left (half), artist on right (half)
	title := track.Title
	artist := track.Artist

	colWidth := contentWidth / 2
	titleWidth := colWidth
	artistWidth := contentWidth - titleWidth

	title = render.TruncateAndPad(title, titleWidth)
	artist = render.TruncateAndPad(artist, artistWidth)

	line := prefix + title + artist + suffix

	// Apply styling based on track state
	style := m.trackStyle(idx, playingIdx)

	return style.Render(line)
}

// trackStyle returns the appropriate style for a track based on its state.
func (m Model) trackStyle(idx, playingIdx int) lipgloss.Style {
	isCursor := idx == m.cursor.Pos() && m.IsFocused()
	isPlaying := idx == playingIdx
	isPlayed := idx < playingIdx

	switch {
	case isCursor && isPlaying:
		return cursorStyle.Inherit(playingStyle)
	case isCursor && isPlayed:
		return cursorStyle.Inherit(dimmedStyle)
	case isCursor:
		return cursorStyle
	case isPlaying:
		return playingStyle
	case isPlayed:
		return dimmedStyle
	default:
		return trackStyle
	}
}
