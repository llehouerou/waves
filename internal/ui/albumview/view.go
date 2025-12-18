package albumview

import (
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

const (
	artistColumnWidth = 30
	yearColumnWidth   = 6     // "2024  " with padding
	albumIndent       = "   " // 3 spaces
	arrowDown         = "↓"
	arrowUp           = "↑"
)

// Group header styles for different levels
func groupHeaderStyles() []lipgloss.Style {
	t := styles.T()
	return []lipgloss.Style{
		lipgloss.NewStyle().Bold(true).Foreground(t.Primary),   // Level 0: Primary
		lipgloss.NewStyle().Bold(true).Foreground(t.Secondary), // Level 1: Secondary
		lipgloss.NewStyle().Bold(true).Foreground(t.Success),   // Level 2: Success
	}
}

func artistStyle() lipgloss.Style {
	return styles.T().S().Base
}

func albumNameStyle() lipgloss.Style {
	return styles.T().S().Muted
}

func yearStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func cursorStyle() lipgloss.Style {
	return styles.T().S().Cursor
}

func dimStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

// Header styles
func headerTitleStyle() lipgloss.Style {
	return styles.T().S().Title
}

func headerKeyStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func headerValueStyle() lipgloss.Style {
	return styles.T().S().Playing
}

func headerSepStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

// View renders the album view.
func (m Model) View() string {
	if m.Width() == 0 || m.Height() == 0 {
		return ""
	}

	// Account for border (2 chars each side)
	innerWidth := m.Width() - 2
	innerHeight := m.Height() - 2
	listHeight := m.listHeight()

	// Header
	header := m.renderHeader(innerWidth)
	separator := render.Separator(innerWidth)

	// Album list
	albumList := m.renderAlbumList(innerWidth, listHeight)

	content := header + "\n" + separator + "\n" + albumList

	return styles.PanelStyle(m.IsFocused()).
		Width(innerWidth).
		Height(innerHeight).
		Render(content)
}

// renderHeader renders the view header with current settings and key bindings.
func (m Model) renderHeader(width int) string {
	title := headerTitleStyle().Render("Albums")
	sep := headerSepStyle().Render(" │ ")

	// Group section: [og] Group: Month ↓
	groupKey := headerKeyStyle().Render("[og]")
	groupLabel := headerKeyStyle().Render("Group:")
	groupValue := m.groupValueLabel()
	groupSection := groupKey + " " + groupLabel + " " + headerValueStyle().Render(groupValue)

	// Sort section: [os] Sort: Original Date ↓
	sortKey := headerKeyStyle().Render("[os]")
	sortLabel := headerKeyStyle().Render("Sort:")
	sortValue := m.sortValueLabel()
	sortSection := sortKey + " " + sortLabel + " " + headerValueStyle().Render(sortValue)

	// Preset section: [op] Preset: name or (none)
	presetKey := headerKeyStyle().Render("[op]")
	presetLabel := headerKeyStyle().Render("Preset:")
	var presetSection string
	if m.settings.PresetName != "" {
		presetSection = presetKey + " " + presetLabel + " " + headerValueStyle().Render(m.settings.PresetName)
	} else {
		presetSection = presetKey + " " + presetLabel
	}

	header := title + sep + groupSection + sep + sortSection + sep + presetSection

	// Pad to width using lipgloss (handles styled text correctly)
	headerWidth := lipgloss.Width(header)
	if headerWidth < width {
		header += strings.Repeat(" ", width-headerWidth)
	}

	return header
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
			isCursor := i == cursorPos && m.IsFocused()
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
	lines = append(lines, dimStyle().Render(centered))

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
	headerStyles := groupHeaderStyles()
	style := headerStyles[item.HeaderLevel%len(headerStyles)]
	return style.Render(line)
}

// renderAlbumLine renders a single album line with columns.
// Format: [indent]Artist                        Album Name                    Year
// The year column is shown when not grouped by release-related date (Best/Original/Release).
func (m Model) renderAlbumLine(album *library.AlbumEntry, width int, isCursor bool) string {
	indentWidth := len(albumIndent)
	availableWidth := width - indentWidth
	showYear := !m.isGroupedByReleaseDate()

	// Artist column (fixed width)
	artist := album.AlbumArtist
	if m.isGroupedByArtist() {
		artist = "" // Don't repeat artist when grouped by artist
	}
	artistCol := render.TruncateAndPad(artist, artistColumnWidth)

	// Calculate widths (artist column + 1 for padding between artist and album)
	usedWidth := artistColumnWidth + 1
	if showYear {
		usedWidth += yearColumnWidth
	}

	// Album column (remaining width)
	albumColWidth := max(availableWidth-usedWidth, 0)
	albumCol := render.TruncateAndPad(album.Album, albumColWidth)

	// Year column (fixed width at end, only when not grouped by release date)
	var yearCol string
	if showYear {
		year := extractYear(album.BestDate())
		yearCol = render.TruncateAndPad(year, yearColumnWidth)
	}

	// Apply styles
	if isCursor {
		// When cursor, use cursor background for the whole line
		line := albumIndent + artistCol + " " + albumCol + yearCol
		return cursorStyle().Render(line)
	}

	// Normal rendering with different colors per column
	if showYear {
		return albumIndent + artistStyle().Render(artistCol) + " " + albumNameStyle().Render(albumCol) + yearStyle().Render(yearCol)
	}
	return albumIndent + artistStyle().Render(artistCol) + " " + albumNameStyle().Render(albumCol)
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

// isGroupedByReleaseDate returns true if grouped by time using a release-related date
// (Best, Original, or Release date), not Added date.
func (m Model) isGroupedByReleaseDate() bool {
	if !m.isGroupedByTime() {
		return false
	}
	// Show year column when grouped by Added date (it's not the release year)
	return m.settings.GroupDateField != DateFieldAdded
}

// groupValueLabel returns a concise label for the current grouping with arrow.
func (m Model) groupValueLabel() string {
	if len(m.settings.GroupFields) == 0 {
		return "None"
	}

	// Build field names
	labels := make([]string, len(m.settings.GroupFields))
	for i, f := range m.settings.GroupFields {
		labels[i] = GroupFieldName(f)
	}

	// Add sort direction arrow
	arrow := arrowDown
	if m.settings.GroupSortOrder == SortAsc {
		arrow = arrowUp
	}

	result := strings.Join(labels, " > ") + " " + arrow

	// Add date field info if using date-based grouping
	if m.hasDateBasedGrouping() {
		result += " (" + DateFieldTypeName(m.settings.GroupDateField) + ")"
	}

	return result
}

// sortValueLabel returns a concise label for the current sorting.
func (m Model) sortValueLabel() string {
	if len(m.settings.SortCriteria) == 0 {
		return "Default"
	}

	labels := make([]string, 0, len(m.settings.SortCriteria))
	for _, c := range m.settings.SortCriteria {
		arrow := arrowDown
		if c.Order == SortAsc {
			arrow = arrowUp
		}
		labels = append(labels, SortFieldName(c.Field)+" "+arrow)
	}
	return strings.Join(labels, ", ")
}

// hasDateBasedGrouping returns true if any selected grouping uses date fields.
func (m Model) hasDateBasedGrouping() bool {
	for _, f := range m.settings.GroupFields {
		if f == GroupFieldYear || f == GroupFieldMonth || f == GroupFieldWeek {
			return true
		}
	}
	return false
}

func extractYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return ""
}
