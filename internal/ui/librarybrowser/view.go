package librarybrowser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// View renders the library browser.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	w1, w2, w3 := m.columnWidths()
	colHeight := m.columnHeight()

	artistCol := m.renderArtistColumn(w1, colHeight)
	albumCol := m.renderAlbumColumn(w2, colHeight)
	trackCol := m.renderTrackColumn(w3, colHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, artistCol, albumCol, trackCol)
}

// renderArtistColumn renders the artist list column with border.
func (m Model) renderArtistColumn(width, height int) string {
	isActive := m.activeColumn == ColumnArtists
	return m.renderBorderedColumn(icons.FormatArtist("Artists"), m.renderArtistItems(width, height), width, isActive)
}

// renderAlbumColumn renders the album list column with border.
func (m Model) renderAlbumColumn(width, height int) string {
	isActive := m.activeColumn == ColumnAlbums
	return m.renderBorderedColumn(icons.FormatAlbum("Albums"), m.renderAlbumItems(width, height), width, isActive)
}

// renderTrackColumn renders the track list column with border.
func (m Model) renderTrackColumn(width, height int) string {
	isActive := m.activeColumn == ColumnTracks
	return m.renderBorderedColumn(icons.FormatAudio("Tracks"), m.renderTrackItems(width, height), width, isActive)
}

// renderBorderedColumn wraps content lines in a bordered box with a title.
func (m Model) renderBorderedColumn(title string, lines []string, width int, active bool) string {
	t := styles.T()

	borderColor := t.Border
	if active && m.focused {
		borderColor = t.BorderFocus
	}

	// Render the title as the first content line (style after pad to avoid ANSI truncation)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(borderColor)
	titleLine := titleStyle.Render(render.TruncateAndPad(title, width))

	content := titleLine + "\n" + render.EmptyLine(width) + "\n" + strings.Join(lines, "\n") + "\n" + render.EmptyLine(width)

	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width)

	return style.Render(content)
}

// styleItem applies the appropriate style to a line based on column state.
func (m Model) styleItem(line string, isCursor, isActive bool) string {
	t := styles.T()

	switch {
	case isCursor && isActive && m.focused:
		return t.S().Cursor.Render(line)
	case isCursor:
		return t.S().Base.Render(line)
	case isActive:
		return t.S().Base.Render(line)
	default:
		return t.S().Muted.Render(line)
	}
}

// styleItemText applies foreground color only (no background) for inline styling.
func (m Model) styleItemText(text string, isCursor, isActive bool) string {
	t := styles.T()

	switch {
	case isCursor && isActive && m.focused:
		return lipgloss.NewStyle().Foreground(t.FgBase).Render(text)
	case isCursor:
		return t.S().Base.Render(text)
	case isActive:
		return t.S().Base.Render(text)
	default:
		return t.S().Muted.Render(text)
	}
}

// styleItemBg applies only the background style to a pre-styled line.
func (m Model) styleItemBg(line string, isCursor, isActive bool) string {
	if isCursor && isActive && m.focused {
		return lipgloss.NewStyle().Background(styles.T().BgCursor).Render(line)
	}
	return line
}

// renderArtistItems renders the artist list items.
func (m Model) renderArtistItems(width, height int) []string {
	isActive := m.activeColumn == ColumnArtists
	lines := make([]string, height)

	for i := range height {
		idx := i + m.artistCursor.Offset()
		if idx >= len(m.artists) {
			lines[i] = render.EmptyLine(width)
			continue
		}

		isCursor := idx == m.artistCursor.Pos()
		name := render.Truncate(m.artists[idx], width-2)

		prefix := "  "
		if isCursor && isActive {
			prefix = "> "
		}

		line := render.Pad(prefix+name, width)
		lines[i] = m.styleItem(line, isCursor, isActive)
	}

	return lines
}

// renderAlbumItems renders the album list items.
func (m Model) renderAlbumItems(width, height int) []string {
	isActive := m.activeColumn == ColumnAlbums
	t := styles.T()
	lines := make([]string, height)

	for i := range height {
		idx := i + m.albumCursor.Offset()
		if idx >= len(m.albums) {
			lines[i] = render.EmptyLine(width)
			continue
		}

		isCursor := idx == m.albumCursor.Pos()
		album := m.albums[idx]

		prefix := "  "
		if isCursor && isActive {
			prefix = "> "
		}

		var yearStr string
		if album.Year > 0 {
			yearStr = strconv.Itoa(album.Year)
		}

		// Reserve space: prefix(2) + name + gap(1) + year + trailing(1)
		yearWidth := runewidth.StringWidth(yearStr)
		maxNameWidth := width - 2 // prefix
		if yearWidth > 0 {
			maxNameWidth -= yearWidth + 2 // gap + year + trailing space
		}
		name := render.Truncate(album.Name, maxNameWidth)

		// Build left part (prefix + name) styled normally
		left := prefix + name
		leftStyled := m.styleItemText(left, isCursor, isActive)

		if yearWidth > 0 {
			// Style year in a muted tone with right padding
			yearStyled := t.S().Muted.Render(yearStr) + " "
			line := render.Row(leftStyled, yearStyled, width)
			lines[i] = m.styleItemBg(line, isCursor, isActive)
		} else {
			line := render.Pad(left, width)
			lines[i] = m.styleItem(line, isCursor, isActive)
		}
	}

	return lines
}

// renderTrackItems renders the track list items.
func (m Model) renderTrackItems(width, height int) []string {
	isActive := m.activeColumn == ColumnTracks
	favIcon := icons.Favorite()
	favIconWidth := runewidth.StringWidth(favIcon)
	lines := make([]string, height)

	for i := range height {
		idx := i + m.trackCursor.Offset()
		if idx >= len(m.tracks) {
			lines[i] = render.EmptyLine(width)
			continue
		}

		isCursor := idx == m.trackCursor.Pos()
		track := m.tracks[idx]
		name := fmt.Sprintf("%02d. %s", track.TrackNumber, track.Title)
		isFavorite := m.favorites[track.ID]

		// Reserve space for prefix and optional favorite icon
		maxNameWidth := width - 2 // 2 for prefix
		if isFavorite {
			maxNameWidth -= favIconWidth + 1
		}
		name = render.Truncate(name, maxNameWidth)

		prefix := "  "
		if isCursor && isActive {
			prefix = "> "
		}

		line := prefix + name
		if isFavorite {
			currentWidth := runewidth.StringWidth(line)
			padding := width - currentWidth - favIconWidth
			if padding > 0 {
				line = line + strings.Repeat(" ", padding) + favIcon
			} else {
				line = render.Pad(line, width-favIconWidth) + favIcon
			}
		} else {
			line = render.Pad(line, width)
		}

		lines[i] = m.styleItem(line, isCursor, isActive)
	}

	return lines
}
