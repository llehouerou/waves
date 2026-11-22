package download

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/fit"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// releaseColumns holds the pre-computed column values for a release row.
type releaseColumns struct {
	name    string
	tracks  string
	year    string
	country string
	format  string
}

// releaseView is the releases of the release group, as its list.
func (m *Model) releaseView() fit.View {
	return withCut(listView("Select a release (different track counts detected):", m.releaseItems(m.textWidth()), "No releases found"),
		m.releasesCut, "releases")
}

// releaseItems are the rows of the releases, each in width columns: the name
// takes what the other columns leave, and the format gives way to it when it
// alone is too wide.
func (m *Model) releaseItems(width int) [][]string {
	rows := make([]releaseColumns, len(m.releases))
	maxNameW := 0
	maxTracksW := 0
	maxYearW := 0
	maxCountryW := 0
	maxFormatW := 0

	for i := range m.releases {
		r := &m.releases[i]

		rows[i].name = r.Title
		rows[i].tracks = fmt.Sprintf("%d tracks", r.TrackCount)

		if r.Date != "" {
			year := r.Date
			if len(year) > 4 {
				year = year[:4]
			}
			rows[i].year = year
		}

		rows[i].country = r.Country
		rows[i].format = r.Formats

		maxNameW = max(maxNameW, lipgloss.Width(rows[i].name))
		maxTracksW = max(maxTracksW, lipgloss.Width(rows[i].tracks))
		maxYearW = max(maxYearW, lipgloss.Width(rows[i].year))
		maxCountryW = max(maxCountryW, lipgloss.Width(rows[i].country))
		maxFormatW = max(maxFormatW, lipgloss.Width(rows[i].format))
	}

	const colGap = 2
	const prefixW = 2
	// Cap name column so other columns remain visible.
	fixedW := prefixW + colGap + maxTracksW + colGap + maxYearW + colGap + maxCountryW + colGap
	if room := width - fixedW - maxFormatW; room < minNameWidth {
		maxFormatW = max(maxFormatW-(minNameWidth-room), 0)
		for i := range rows {
			rows[i].format = render.Cut(rows[i].format, maxFormatW)
		}
	}
	maxAllowedNameW := max(width-fixedW-maxFormatW, minNameWidth)
	if maxNameW > maxAllowedNameW {
		maxNameW = maxAllowedNameW
		for i := range rows {
			if lipgloss.Width(rows[i].name) > maxNameW {
				rows[i].name = render.Cut(rows[i].name, maxNameW)
			}
		}
	}

	cursorPos := m.releaseCursor.Pos()
	items := make([][]string, len(rows))
	for i := range rows {
		row := &rows[i]

		namePad := colGap + maxNameW - lipgloss.Width(row.name)
		tracksPad := colGap + maxTracksW - lipgloss.Width(row.tracks)
		yearPad := colGap + maxYearW - lipgloss.Width(row.year)
		countryPad := colGap + maxCountryW - lipgloss.Width(row.country)

		plain := row.name + strings.Repeat(" ", namePad) +
			row.tracks + strings.Repeat(" ", tracksPad) +
			row.year + strings.Repeat(" ", yearPad) +
			row.country + strings.Repeat(" ", countryPad) +
			row.format
		styled := styles.T().S().Base.Render(row.name+render.EmptyLine(namePad)) +
			typeStyle().Render(row.tracks+render.EmptyLine(tracksPad)) +
			dimStyle().Render(row.year+render.EmptyLine(yearPad)+row.country+render.EmptyLine(countryPad)+row.format)
		items[i] = []string{listRow(plain, styled, i == cursorPos, width)}
	}
	return items
}
