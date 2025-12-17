package download

import (
	"fmt"
	"strings"

	"github.com/llehouerou/waves/internal/musicbrainz"
)

// renderReleaseResults renders the releases for track count selection.
func (m *Model) renderReleaseResults() string {
	if len(m.releases) == 0 {
		return dimStyle.Render("No releases found")
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render("Select a release (different track counts detected):"))
	b.WriteString("\n\n")

	maxVisible := max(m.height-12, 5)
	start, end := m.releaseCursor.VisibleRange(len(m.releases), maxVisible)
	cursorPos := m.releaseCursor.Pos()

	for i := start; i < end; i++ {
		r := &m.releases[i]
		line := m.formatRelease(r)

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

// formatRelease formats a single release for display.
func (m *Model) formatRelease(r *musicbrainz.Release) string {
	parts := []string{r.Title}

	// Track count (most important)
	parts = append(parts, typeStyle.Render(fmt.Sprintf("[%d tracks]", r.TrackCount)))

	// Date
	if r.Date != "" {
		year := r.Date
		if len(year) > 4 {
			year = year[:4]
		}
		parts = append(parts, fmt.Sprintf("(%s)", year))
	}

	// Country
	if r.Country != "" {
		parts = append(parts, dimStyle.Render("["+r.Country+"]"))
	}

	// Formats (CD, Vinyl, Digital, etc.)
	if r.Formats != "" {
		parts = append(parts, dimStyle.Render(r.Formats))
	}

	return strings.Join(parts, " ")
}
