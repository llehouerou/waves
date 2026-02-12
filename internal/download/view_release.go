package download

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// releaseColumns holds the pre-computed column values for a release row.
type releaseColumns struct {
	name    string
	tracks  string
	year    string
	country string
	format  string
}

// renderReleaseResults renders the releases for track count selection.
func (m *Model) renderReleaseResults() string {
	if len(m.releases) == 0 {
		return dimStyle().Render("No releases found")
	}

	var b strings.Builder
	b.WriteString(dimStyle().Render("Select a release (different track counts detected):"))
	b.WriteString("\n\n")

	maxVisible := max(m.Height()-12, 5)
	start, end := m.releaseCursor.VisibleRange(len(m.releases), maxVisible)
	cursorPos := m.releaseCursor.Pos()

	// Pre-compute column values and measure max widths for visible items.
	rows := make([]releaseColumns, end-start)
	maxNameW := 0
	maxTracksW := 0
	maxYearW := 0
	maxCountryW := 0

	for i := start; i < end; i++ {
		r := &m.releases[i]
		idx := i - start

		rows[idx].name = r.Title
		rows[idx].tracks = fmt.Sprintf("%d tracks", r.TrackCount)

		if r.Date != "" {
			year := r.Date
			if len(year) > 4 {
				year = year[:4]
			}
			rows[idx].year = year
		}

		rows[idx].country = r.Country
		rows[idx].format = r.Formats

		if w := lipgloss.Width(rows[idx].name); w > maxNameW {
			maxNameW = w
		}
		if w := lipgloss.Width(rows[idx].tracks); w > maxTracksW {
			maxTracksW = w
		}
		if w := lipgloss.Width(rows[idx].year); w > maxYearW {
			maxYearW = w
		}
		if w := lipgloss.Width(rows[idx].country); w > maxCountryW {
			maxCountryW = w
		}
	}

	const colGap = 2
	const prefixW = 2
	// Cap name column so other columns remain visible.
	fixedW := prefixW + maxTracksW + maxYearW + maxCountryW + colGap*3
	// Add a rough estimate for format column.
	fixedW += 10 + colGap
	maxAllowedNameW := max(m.Width()-fixedW-4, 10)
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

		nameCol := row.name + strings.Repeat(" ", colGap+maxNameW-lipgloss.Width(row.name))
		tracksCol := row.tracks + strings.Repeat(" ", colGap+maxTracksW-lipgloss.Width(row.tracks))
		yearCol := row.year + strings.Repeat(" ", colGap+maxYearW-lipgloss.Width(row.year))
		countryCol := row.country + strings.Repeat(" ", colGap+maxCountryW-lipgloss.Width(row.country))
		formatCol := row.format

		line := nameCol + tracksCol + yearCol + countryCol + formatCol

		if i == cursorPos {
			b.WriteString(cursorStyle().Render("> "))
			b.WriteString(selectedStyle().Render(line))
		} else {
			b.WriteString("  ")
			styledLine := nameCol +
				typeStyle().Render(tracksCol) +
				dimStyle().Render(yearCol+countryCol+formatCol)
			b.WriteString(styledLine)
		}
		b.WriteString("\n")
	}

	return b.String()
}
