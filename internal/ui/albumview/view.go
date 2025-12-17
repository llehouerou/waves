package albumview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

const (
	artistColumnWidth = 30
	albumIndent       = "   " // 3 spaces
)

var (
	groupHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")) // Cyan for group headers

	artistStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")) // Bright for artist

	albumNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")) // Dimmer for album

	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// View renders the album view.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Account for border (2 chars each side)
	innerWidth := m.width - 2
	innerHeight := m.height - 2
	listHeight := m.listHeight()

	// Header
	header := m.renderHeader(innerWidth)
	separator := render.Separator(innerWidth)

	// Album list
	albumList := m.renderAlbumList(innerWidth, listHeight)

	content := header + "\n" + separator + "\n" + albumList

	return styles.PanelStyle(m.focused).
		Width(innerWidth).
		Height(innerHeight).
		Render(content)
}

// renderHeader renders the view header with current settings.
func (m Model) renderHeader(width int) string {
	groupLabel := m.groupByLabel()
	sortLabel := m.sortLabel()
	text := fmt.Sprintf("Albums (%s, %s)", groupLabel, sortLabel)
	return render.TruncateAndPad(text, width)
}

// renderAlbumList renders the list of albums with groups.
func (m Model) renderAlbumList(width, height int) string {
	if len(m.flatList) == 0 {
		return m.renderEmpty(width, height)
	}

	lines := make([]string, 0, height)

	offset := m.cursor.Offset()
	cursorPos := m.cursor.Pos()
	for i := offset; i < len(m.flatList) && len(lines) < height; i++ {
		item := m.flatList[i]

		if item.IsHeader {
			line := m.renderGroupHeader(item.Header, width)
			lines = append(lines, line)
		} else {
			isCursor := i == cursorPos && m.focused
			line := m.renderAlbumLine(item.Album, width, isCursor)
			lines = append(lines, line)
		}
	}

	// Fill remaining height
	for len(lines) < height {
		lines = append(lines, render.EmptyLine(width))
	}

	return strings.Join(lines, "\n")
}

// renderEmpty renders the empty state.
func (m Model) renderEmpty(width, height int) string {
	lines := make([]string, 0, height)

	// Center message vertically
	emptyLines := height / 2
	for range emptyLines {
		lines = append(lines, render.EmptyLine(width))
	}

	msg := "No albums in library"
	centered := render.TruncateAndPad(msg, width)
	lines = append(lines, dimStyle.Render(centered))

	for len(lines) < height {
		lines = append(lines, render.EmptyLine(width))
	}

	return strings.Join(lines, "\n")
}

// renderGroupHeader renders a group header line with extending decoration.
func (m Model) renderGroupHeader(header string, width int) string {
	// Format: "── December 2024 ────────────────"
	prefix := "── "
	suffix := " "
	labelWidth := lipgloss.Width(prefix) + len(header) + lipgloss.Width(suffix)

	// Fill remaining width with ─
	remaining := max(width-labelWidth, 0)
	line := prefix + header + suffix + strings.Repeat("─", remaining)

	return groupHeaderStyle.Render(line)
}

// renderAlbumLine renders a single album line with two-column layout.
// Format: [indent]Artist                        Album Name
func (m Model) renderAlbumLine(album *library.AlbumEntry, width int, isCursor bool) string {
	indentWidth := len(albumIndent)
	availableWidth := width - indentWidth

	// Artist column (fixed width)
	artist := album.AlbumArtist
	if m.settings.GroupBy == GroupByArtist {
		artist = "" // Don't repeat artist when grouped by artist
	}
	artistCol := render.TruncateAndPad(artist, artistColumnWidth)

	// Album column (remaining width)
	albumColWidth := max(availableWidth-artistColumnWidth, 0)

	// Build album text - add year if not grouped by time
	albumText := album.Album
	if m.settings.GroupBy != GroupByYear &&
		m.settings.GroupBy != GroupByWeek &&
		m.settings.GroupBy != GroupByMonth {
		year := extractYear(album.BestDate())
		if year != "" {
			albumText = fmt.Sprintf("%s (%s)", album.Album, year)
		}
	}
	albumCol := render.TruncateAndPad(albumText, albumColWidth)

	// Apply styles
	if isCursor {
		// When cursor, use cursor background for the whole line
		line := albumIndent + artistCol + albumCol
		return cursorStyle.Render(line)
	}

	// Normal rendering with different colors per column
	return albumIndent + artistStyle.Render(artistCol) + albumNameStyle.Render(albumCol)
}

// groupByLabel returns a human-readable label for the current grouping.
func (m Model) groupByLabel() string {
	switch m.settings.GroupBy {
	case GroupByWeek:
		return "by week"
	case GroupByMonth:
		return "by month"
	case GroupByYear:
		return "by year"
	case GroupByArtist:
		return "by artist"
	case GroupByGenre:
		return "by genre"
	case GroupByAddedAt:
		return "by added"
	case GroupByNone:
		return "all"
	}
	return ""
}

// sortLabel returns a human-readable label for the current sorting.
func (m Model) sortLabel() string {
	order := "newest"
	if m.settings.SortOrder == SortAsc {
		order = "oldest"
	}

	switch m.settings.SortBy {
	case SortByOriginalDate:
		return order + " first"
	case SortByReleaseDate:
		return "release " + order
	case SortByAddedAt:
		return "recently added"
	case SortByArtist:
		return "by artist"
	case SortByAlbum:
		return "by album"
	}
	return ""
}

func extractYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return ""
}
