package download

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
)

// releaseGroupColumns holds the pre-computed column values for a release group row.
type releaseGroupColumns struct {
	name      string
	year      string
	typeLabel string
	inLibrary bool
}

// renderReleaseGroupResults renders the release groups grouped by type.
func (m *Model) renderReleaseGroupResults() string {
	if len(m.releaseGroups) == 0 {
		return dimStyle().Render("No releases found")
	}

	var b strings.Builder
	b.WriteString(dimStyle().Render("Select a release:"))
	b.WriteString("\n\n")

	maxVisible := max(m.Height()-12, 5)
	start, end := m.releaseGroupCursor.VisibleRange(len(m.releaseGroups), maxVisible)
	cursorPos := m.releaseGroupCursor.Pos()

	// Pre-compute column values and measure max widths for visible items.
	rows := make([]releaseGroupColumns, end-start)
	maxNameW := 0
	maxYearW := 0
	maxTypeW := 0

	for i := start; i < end; i++ {
		rg := &m.releaseGroups[i]
		idx := i - start
		rows[idx].inLibrary = m.IsInLibrary(*rg)

		// Name column (with library icon prefix).
		name := rg.Title
		if rows[idx].inLibrary {
			name = icons.InLibrary() + " " + name
		}
		rows[idx].name = name

		// Year column.
		if rg.FirstRelease != "" {
			year := rg.FirstRelease
			if len(year) > 4 {
				year = year[:4]
			}
			rows[idx].year = year
		}

		// Type column.
		typeLabel := rg.PrimaryType
		if len(rg.SecondaryTypes) > 0 {
			typeLabel += "+" + strings.Join(rg.SecondaryTypes, "+")
		}
		rows[idx].typeLabel = typeLabel

		if w := lipgloss.Width(rows[idx].name); w > maxNameW {
			maxNameW = w
		}
		if w := lipgloss.Width(rows[idx].year); w > maxYearW {
			maxYearW = w
		}
		if w := lipgloss.Width(rows[idx].typeLabel); w > maxTypeW {
			maxTypeW = w
		}
	}

	const colGap = 2
	// cursor prefix "  " or "> " = 2 chars
	const prefixW = 2
	// Cap name column so year + type columns always remain visible.
	fixedW := prefixW + maxYearW + maxTypeW + colGap*2
	maxAllowedNameW := max(m.Width()-fixedW-4, 10) // 4 for popup padding
	if maxNameW > maxAllowedNameW {
		maxNameW = maxAllowedNameW
		for i := range rows {
			if lipgloss.Width(rows[i].name) > maxNameW {
				rows[i].name = truncateName(rows[i].name, maxNameW)
			}
		}
	}

	for i := start; i < end; i++ {
		row := &rows[i-start]

		// Build the line with aligned columns.
		nameCol := row.name + strings.Repeat(" ", colGap+maxNameW-lipgloss.Width(row.name))
		yearCol := row.year + strings.Repeat(" ", colGap+maxYearW-lipgloss.Width(row.year))
		typeCol := row.typeLabel

		line := nameCol + yearCol + typeCol

		if i == cursorPos {
			b.WriteString(cursorStyle().Render("> "))
			b.WriteString(selectedStyle().Render(line))
		} else {
			b.WriteString("  ")
			if row.inLibrary {
				b.WriteString(dimStyle().Render(line))
			} else {
				// Apply type styling only to the type column.
				styledLine := nameCol + dimStyle().Render(yearCol) + typeStyle().Render(typeCol)
				b.WriteString(styledLine)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
