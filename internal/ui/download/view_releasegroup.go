package download

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui/fit"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// releaseGroupColumns holds the pre-computed column values for a release group row.
type releaseGroupColumns struct {
	name      string
	year      string
	typeLabel string
	inLibrary bool
}

// minNameWidth is the width a name column keeps before the row is cut.
const minNameWidth = 10

// releaseGroupView is the release groups, as its list.
func (m *Model) releaseGroupView() fit.View {
	return listView("Select a release:", m.releaseGroupItems(m.textWidth()), "No releases found")
}

// releaseGroupItems are the rows of the release groups, each in width
// columns: the name takes what the year and the type leave.
func (m *Model) releaseGroupItems(width int) [][]string {
	rows := make([]releaseGroupColumns, len(m.releaseGroups))
	maxNameW := 0
	maxYearW := 0
	maxTypeW := 0

	for i := range m.releaseGroups {
		rg := &m.releaseGroups[i]
		rows[i].inLibrary = m.IsInLibrary(*rg)

		// Name column (with library icon prefix).
		name := rg.Title
		if rows[i].inLibrary {
			name = icons.InLibrary() + " " + name
		}
		rows[i].name = name

		// Year column.
		if rg.FirstRelease != "" {
			year := rg.FirstRelease
			if len(year) > 4 {
				year = year[:4]
			}
			rows[i].year = year
		}

		// Type column.
		typeLabel := rg.PrimaryType
		if len(rg.SecondaryTypes) > 0 {
			typeLabel += "+" + strings.Join(rg.SecondaryTypes, "+")
		}
		rows[i].typeLabel = typeLabel

		maxNameW = max(maxNameW, lipgloss.Width(rows[i].name))
		maxYearW = max(maxYearW, lipgloss.Width(rows[i].year))
		maxTypeW = max(maxTypeW, lipgloss.Width(rows[i].typeLabel))
	}

	const colGap = 2
	// cursor prefix "  " or "> " = 2 chars
	const prefixW = 2
	// Cap name column so year + type columns always remain visible.
	fixedW := prefixW + colGap + maxYearW + colGap + maxTypeW
	maxAllowedNameW := max(width-fixedW, minNameWidth)
	if maxNameW > maxAllowedNameW {
		maxNameW = maxAllowedNameW
		for i := range rows {
			if lipgloss.Width(rows[i].name) > maxNameW {
				rows[i].name = render.Cut(rows[i].name, maxNameW)
			}
		}
	}

	cursorPos := m.releaseGroupCursor.Pos()
	items := make([][]string, len(rows))
	for i := range rows {
		row := &rows[i]

		// Build columns with plain padding (for cursor/selected) and styled padding (for normal).
		namePad := colGap + maxNameW - lipgloss.Width(row.name)
		yearPad := colGap + maxYearW - lipgloss.Width(row.year)

		plain := row.name + strings.Repeat(" ", namePad) + row.year + strings.Repeat(" ", yearPad) + row.typeLabel
		styled := dimStyle().Render(plain)
		if !row.inLibrary {
			styled = styles.T().S().Base.Render(row.name+render.EmptyLine(namePad)) +
				dimStyle().Render(row.year+render.EmptyLine(yearPad)) +
				typeStyle().Render(row.typeLabel)
		}
		items[i] = []string{listRow(plain, styled, i == cursorPos, width)}
	}
	return items
}
