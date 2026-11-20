package download

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/fit"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// searchView is the search input: a line the user types in.
func (m *Model) searchView() fit.View {
	return fit.View{Above: []fit.Line{
		{Text: dimStyle().Render("Search for an artist:")},
		fit.Keep(m.searchInput.View()),
	}}
}

// artistColumns holds the pre-computed column values for an artist row.
type artistColumns struct {
	name    string
	disamb  string
	typeCol string
	country string
	years   string
}

// artistView is the artist search results, the artists as its list.
func (m *Model) artistView() fit.View {
	return listView("Select an artist:", m.artistItems(m.textWidth()), "No artists found")
}

// artistItems are the rows of the artists found, each in width columns: the
// name and its disambiguation take what the other columns leave.
func (m *Model) artistItems(width int) [][]string {
	rows := make([]artistColumns, len(m.artistResults))
	maxCombinedW := 0
	maxTypeW := 0
	maxCountryW := 0
	maxYearsW := 0

	for i := range m.artistResults {
		a := &m.artistResults[i]

		rows[i].name = a.Name
		rows[i].disamb = a.Disambiguation
		rows[i].typeCol = a.Type
		rows[i].country = a.Country

		if a.BeginYear != "" {
			years := a.BeginYear + "-"
			if a.EndYear != "" {
				years += a.EndYear
			}
			rows[i].years = years
		}

		maxCombinedW = max(maxCombinedW, combinedWidth(rows[i].name, rows[i].disamb))
		maxTypeW = max(maxTypeW, lipgloss.Width(rows[i].typeCol))
		maxCountryW = max(maxCountryW, lipgloss.Width(rows[i].country))
		maxYearsW = max(maxYearsW, lipgloss.Width(rows[i].years))
	}

	const colGap = 2
	const prefixW = 2
	// The other columns are never truncated: the name gives way to them.
	fixedColsW := prefixW + colGap + maxTypeW + colGap + maxCountryW + colGap + maxYearsW
	maxAllowedCombinedW := max(width-fixedColsW, minNameWidth)
	if maxCombinedW > maxAllowedCombinedW {
		maxCombinedW = maxAllowedCombinedW
		truncateArtistRows(rows, maxCombinedW)
	}

	cursorPos := m.artistCursor.Pos()
	items := make([][]string, len(rows))
	for i := range rows {
		row := &rows[i]

		// Build the combined name+disamb text, then pad to fixed column width.
		combined := row.name
		if row.disamb != "" {
			combined += " " + row.disamb
		}
		combinedPad := colGap + maxCombinedW - lipgloss.Width(combined)
		typePad := colGap + maxTypeW - lipgloss.Width(row.typeCol)

		// The row as the cursor shows it.
		plain := combined + strings.Repeat(" ", combinedPad) +
			row.typeCol + strings.Repeat(" ", typePad) +
			fmt.Sprintf("%-*s%s", maxCountryW+colGap, row.country, row.years)

		// Name in normal style, disamb in dim.
		styled := styles.T().S().Base.Render(row.name)
		if row.disamb != "" {
			styled += render.EmptyLine(1) + dimStyle().Render(row.disamb)
		}
		styled += render.EmptyLine(combinedPad) +
			typeStyle().Render(row.typeCol+render.EmptyLine(typePad)) +
			dimStyle().Render(fmt.Sprintf("%-*s%s", maxCountryW+colGap, row.country, row.years))

		items[i] = []string{listRow(plain, styled, i == cursorPos, width)}
	}
	return items
}

// combinedWidth returns the display width of "name disamb" (with space) or just "name".
func combinedWidth(name, disamb string) int {
	w := lipgloss.Width(name)
	if disamb != "" {
		w += 1 + lipgloss.Width(disamb)
	}
	return w
}

// truncateArtistRows truncates disamb first, then name, so each row fits within maxW.
func truncateArtistRows(rows []artistColumns, maxW int) {
	for i := range rows {
		w := combinedWidth(rows[i].name, rows[i].disamb)
		if w <= maxW {
			continue
		}
		// Try dropping disamb entirely.
		nameW := lipgloss.Width(rows[i].name)
		if nameW <= maxW {
			// Truncate disamb to fit: maxW - nameW - 1 (space separator).
			rows[i].disamb = render.Cut(rows[i].disamb, max(maxW-nameW-1, 0))
			continue
		}
		// Name alone is too long; drop disamb and truncate name.
		rows[i].disamb = ""
		rows[i].name = render.Cut(rows[i].name, maxW)
	}
}
