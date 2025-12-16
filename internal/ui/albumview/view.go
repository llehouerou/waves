package albumview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")) // Cyan for group headers

	albumStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252"))

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

	for i := m.offset; i < len(m.flatList) && len(lines) < height; i++ {
		item := m.flatList[i]

		if item.IsHeader {
			line := m.renderGroupHeader(item.Header, width)
			lines = append(lines, line)
		} else {
			isCursor := i == m.cursor && m.focused
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

// renderGroupHeader renders a group header line.
func (m Model) renderGroupHeader(header string, width int) string {
	// Format: "-- Dec 9 - Dec 15, 2024 --"
	text := "-- " + header + " --"
	padded := render.TruncateAndPad(text, width)
	return headerStyle.Render(padded)
}

// renderAlbumLine renders a single album line.
// Format: Artist - Album - Year (dynamically hides grouped field)
func (m Model) renderAlbumLine(album *library.AlbumEntry, width int, isCursor bool) string {
	var parts []string

	// Always show artist unless grouped by artist
	if m.settings.GroupBy != GroupByArtist {
		parts = append(parts, album.AlbumArtist)
	}

	// Always show album name
	parts = append(parts, album.Album)

	// Show year unless grouped by year/week/month
	if m.settings.GroupBy != GroupByYear &&
		m.settings.GroupBy != GroupByWeek &&
		m.settings.GroupBy != GroupByMonth {
		year := extractYear(album.BestDate())
		if year != "" {
			parts = append(parts, year)
		}
	}

	line := strings.Join(parts, " - ")
	line = render.TruncateAndPad(line, width)

	if isCursor {
		return cursorStyle.Render(line)
	}
	return albumStyle.Render(line)
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
