package download

import (
	"strings"

	"github.com/llehouerou/waves/internal/musicbrainz"
)

// renderSearchSection renders the search input.
func (m *Model) renderSearchSection() string {
	var b strings.Builder
	b.WriteString(dimStyle.Render("Search for an artist:"))
	b.WriteString("\n")
	b.WriteString(m.searchInput.View())
	return b.String()
}

// renderArtistResults renders the artist search results.
func (m *Model) renderArtistResults() string {
	if len(m.artistResults) == 0 {
		return dimStyle.Render("No artists found")
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render("Select an artist:"))
	b.WriteString("\n\n")

	maxVisible := max(m.Height()-12, 5)
	start, end := m.artistCursor.VisibleRange(len(m.artistResults), maxVisible)
	cursorPos := m.artistCursor.Pos()

	for i := start; i < end; i++ {
		a := &m.artistResults[i]
		line := m.formatArtist(a)

		if i == cursorPos {
			b.WriteString(cursorStyle.Render("> "))
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString("  ")
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// formatArtist formats a single artist.
func (m *Model) formatArtist(a *musicbrainz.Artist) string {
	parts := []string{a.Name}

	// Add disambiguation if present (e.g., "British rock band")
	if a.Disambiguation != "" {
		parts = append(parts, dimStyle.Render("("+a.Disambiguation+")"))
	}

	// Add type (Person, Group, etc.)
	if a.Type != "" {
		parts = append(parts, typeStyle.Render("["+a.Type+"]"))
	}

	// Add country
	if a.Country != "" {
		parts = append(parts, dimStyle.Render("["+a.Country+"]"))
	}

	// Add life span (e.g., "1965-" or "1965-2020")
	if a.BeginYear != "" {
		lifeSpan := a.BeginYear + "-"
		if a.EndYear != "" {
			lifeSpan += a.EndYear
		}
		parts = append(parts, dimStyle.Render(lifeSpan))
	}

	return strings.Join(parts, " ")
}
