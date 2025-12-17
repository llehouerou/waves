package albumview

import (
	"fmt"
	"slices"
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
	// Group header styles for different levels
	groupHeaderStyles = []lipgloss.Style{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),  // Level 0: Cyan
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141")), // Level 1: Purple
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("179")), // Level 2: Gold
	}

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
			line := m.renderGroupHeader(item, width)
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
func (m Model) renderGroupHeader(item AlbumItem, width int) string {
	// Indent based on level
	indent := strings.Repeat("  ", item.HeaderLevel)

	// Format: "── December 2024 ────────────────"
	prefix := "── "
	suffix := " "
	labelWidth := lipgloss.Width(indent) + lipgloss.Width(prefix) + len(item.Header) + lipgloss.Width(suffix)

	// Fill remaining width with ─
	remaining := max(width-labelWidth, 0)
	line := indent + prefix + item.Header + suffix + strings.Repeat("─", remaining)

	// Use style based on level
	style := groupHeaderStyles[item.HeaderLevel%len(groupHeaderStyles)]
	return style.Render(line)
}

// renderAlbumLine renders a single album line with two-column layout.
// Format: [indent]Artist                        Album Name
func (m Model) renderAlbumLine(album *library.AlbumEntry, width int, isCursor bool) string {
	indentWidth := len(albumIndent)
	availableWidth := width - indentWidth

	// Artist column (fixed width)
	artist := album.AlbumArtist
	if m.isGroupedByArtist() {
		artist = "" // Don't repeat artist when grouped by artist
	}
	artistCol := render.TruncateAndPad(artist, artistColumnWidth)

	// Album column (remaining width)
	albumColWidth := max(availableWidth-artistColumnWidth, 0)

	// Build album text - add year if not grouped by time
	albumText := album.Album
	if !m.isGroupedByTime() {
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

// isGroupedByArtist returns true if any grouping level is by artist.
func (m Model) isGroupedByArtist() bool {
	return slices.Contains(m.settings.GroupFields, GroupFieldArtist)
}

// isGroupedByTime returns true if any grouping level is by time (year, month, week).
func (m Model) isGroupedByTime() bool {
	for _, f := range m.settings.GroupFields {
		if f == GroupFieldYear || f == GroupFieldMonth || f == GroupFieldWeek {
			return true
		}
	}
	return false
}

// groupByLabel returns a human-readable label for the current grouping.
func (m Model) groupByLabel() string {
	if len(m.settings.GroupFields) == 0 {
		return "all"
	}

	labels := make([]string, len(m.settings.GroupFields))
	for i, f := range m.settings.GroupFields {
		labels[i] = strings.ToLower(GroupFieldName(f))
	}
	return "by " + strings.Join(labels, " > ")
}

// sortLabel returns a human-readable label for the current sorting.
func (m Model) sortLabel() string {
	if len(m.settings.SortCriteria) == 0 {
		return "default"
	}

	labels := make([]string, 0, len(m.settings.SortCriteria))
	for _, c := range m.settings.SortCriteria {
		label := strings.ToLower(SortFieldName(c.Field))
		if c.Order == SortAsc {
			label += " asc"
		} else {
			label += " desc"
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, ", ")
}

func extractYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return ""
}
