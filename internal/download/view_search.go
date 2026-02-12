package download

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderSearchSection renders the search input.
func (m *Model) renderSearchSection() string {
	var b strings.Builder
	b.WriteString(dimStyle().Render("Search for an artist:"))
	b.WriteString("\n")
	b.WriteString(m.searchInput.View())
	return b.String()
}

// artistColumns holds the pre-computed column values for an artist row.
type artistColumns struct {
	name    string
	disamb  string
	typeCol string
	country string
	years   string
}

// renderArtistResults renders the artist search results.
func (m *Model) renderArtistResults() string {
	if len(m.artistResults) == 0 {
		return dimStyle().Render("No artists found")
	}

	var b strings.Builder
	b.WriteString(dimStyle().Render("Select an artist:"))
	b.WriteString("\n\n")

	maxVisible := max(m.Height()-12, 5)
	start, end := m.artistCursor.VisibleRange(len(m.artistResults), maxVisible)
	cursorPos := m.artistCursor.Pos()

	// Pre-compute column values and measure max widths for visible items.
	rows := make([]artistColumns, end-start)
	maxCombinedW := 0
	maxTypeW := 0
	maxCountryW := 0

	for i := start; i < end; i++ {
		a := &m.artistResults[i]
		idx := i - start

		rows[idx].name = a.Name
		rows[idx].disamb = a.Disambiguation
		rows[idx].typeCol = a.Type
		rows[idx].country = a.Country

		if a.BeginYear != "" {
			years := a.BeginYear + "-"
			if a.EndYear != "" {
				years += a.EndYear
			}
			rows[idx].years = years
		}

		if w := combinedWidth(rows[idx].name, rows[idx].disamb); w > maxCombinedW {
			maxCombinedW = w
		}
		if w := lipgloss.Width(rows[idx].typeCol); w > maxTypeW {
			maxTypeW = w
		}
		if w := lipgloss.Width(rows[idx].country); w > maxCountryW {
			maxCountryW = w
		}
	}

	const colGap = 2
	const prefixW = 2
	// Fixed-width columns that are never truncated.
	fixedColsW := prefixW + maxTypeW + maxCountryW + colGap*2
	// Add lifespan column width estimate (e.g. "1965-2020" = 9).
	fixedColsW += 9 + colGap
	maxAllowedCombinedW := max(m.Width()-fixedColsW-4, 10)
	if maxCombinedW > maxAllowedCombinedW {
		maxCombinedW = maxAllowedCombinedW
		truncateArtistRows(rows, maxCombinedW)
	}

	for i := start; i < end; i++ {
		row := &rows[i-start]

		// Build the combined name+disamb text, then pad to fixed column width.
		combined := row.name
		if row.disamb != "" {
			combined += " " + row.disamb
		}
		pad := strings.Repeat(" ", colGap+maxCombinedW-lipgloss.Width(combined))

		typeCol := row.typeCol + strings.Repeat(" ", colGap+maxTypeW-lipgloss.Width(row.typeCol))
		yearsCol := row.years

		if i == cursorPos {
			b.WriteString(cursorStyle().Render("> "))
			b.WriteString(selectedStyle().Render(row.name))
			if row.disamb != "" {
				b.WriteString(" " + dimStyle().Render(row.disamb))
			}
			b.WriteString(pad)
			b.WriteString(typeStyle().Render(typeCol))
			b.WriteString(dimStyle().Render(fmt.Sprintf("%-*s%s", maxCountryW+colGap, row.country, yearsCol)))
		} else {
			b.WriteString("  ")
			// Name in normal style, disamb in dim.
			styledCombined := row.name
			if row.disamb != "" {
				styledCombined += " " + dimStyle().Render(row.disamb)
			}
			styledCombined += pad

			b.WriteString(styledCombined)
			b.WriteString(typeStyle().Render(typeCol))
			b.WriteString(dimStyle().Render(fmt.Sprintf("%-*s%s", maxCountryW+colGap, row.country, yearsCol)))
		}
		b.WriteString("\n")
	}

	return b.String()
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
			rows[i].disamb = truncateName(rows[i].disamb, max(maxW-nameW-1, 0))
			continue
		}
		// Name alone is too long; drop disamb and truncate name.
		rows[i].disamb = ""
		rows[i].name = truncateName(rows[i].name, maxW)
	}
}
