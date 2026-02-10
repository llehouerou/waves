package librarybrowser

import (
	"fmt"
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

	colWidth := m.columnWidth()
	colHeight := m.columnHeight()

	artistCol := m.renderArtistColumn(colWidth, colHeight)
	albumCol := m.renderAlbumColumn(colWidth, colHeight)
	trackCol := m.renderTrackColumn(colWidth, colHeight)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, artistCol, albumCol, trackCol)
	description := m.renderDescription()

	return lipgloss.JoinVertical(lipgloss.Left, columns, description)
}

// renderArtistColumn renders the artist list column with border.
func (m Model) renderArtistColumn(width, height int) string {
	isActive := m.activeColumn == ColumnArtists
	return m.renderBorderedColumn("Artists", m.renderArtistItems(width, height), width, isActive)
}

// renderAlbumColumn renders the album list column with border.
func (m Model) renderAlbumColumn(width, height int) string {
	isActive := m.activeColumn == ColumnAlbums
	return m.renderBorderedColumn("Albums", m.renderAlbumItems(width, height), width, isActive)
}

// renderTrackColumn renders the track list column with border.
func (m Model) renderTrackColumn(width, height int) string {
	isActive := m.activeColumn == ColumnTracks
	return m.renderBorderedColumn("Tracks", m.renderTrackItems(width, height), width, isActive)
}

// renderBorderedColumn wraps content lines in a bordered box with a title.
func (m Model) renderBorderedColumn(title string, lines []string, width int, active bool) string {
	t := styles.T()

	borderColor := t.Border
	if active && m.focused {
		borderColor = t.BorderFocus
	}

	// Render the title as the first content line
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(borderColor)
	titleLine := render.TruncateAndPad(titleStyle.Render(title), width)

	content := titleLine + "\n" + strings.Join(lines, "\n")

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
	case isActive:
		return t.S().Base.Render(line)
	default:
		return t.S().Muted.Render(line)
	}
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
		name := icons.FormatArtist(m.artists[idx])
		name = render.Truncate(name, width-2)

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
	lines := make([]string, height)

	for i := range height {
		idx := i + m.albumCursor.Offset()
		if idx >= len(m.albums) {
			lines[i] = render.EmptyLine(width)
			continue
		}

		isCursor := idx == m.albumCursor.Pos()
		album := m.albums[idx]
		name := album.Name
		if album.Year > 0 {
			name = fmt.Sprintf("%s (%d)", name, album.Year)
		}
		name = icons.FormatAlbum(name)
		name = render.Truncate(name, width-2)

		prefix := "  "
		if isCursor && isActive {
			prefix = "> "
		}

		line := render.Pad(prefix+name, width)
		lines[i] = m.styleItem(line, isCursor, isActive)
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
