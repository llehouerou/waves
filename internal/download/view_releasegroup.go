package download

import (
	"fmt"
	"strings"

	"github.com/llehouerou/waves/internal/musicbrainz"
)

// renderReleaseGroupResults renders the release groups grouped by type.
func (m *Model) renderReleaseGroupResults() string {
	if len(m.releaseGroups) == 0 {
		return dimStyle.Render("No releases found")
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render("Select a release:"))
	b.WriteString("\n\n")

	maxVisible := max(m.Height()-12, 5)
	start, end := m.releaseGroupCursor.VisibleRange(len(m.releaseGroups), maxVisible)
	cursorPos := m.releaseGroupCursor.Pos()

	for i := start; i < end; i++ {
		rg := &m.releaseGroups[i]
		line := m.formatReleaseGroup(rg)

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

// formatReleaseGroup formats a single release group.
func (m *Model) formatReleaseGroup(rg *musicbrainz.ReleaseGroup) string {
	parts := []string{rg.Title}

	if rg.FirstRelease != "" {
		year := rg.FirstRelease
		if len(year) > 4 {
			year = year[:4]
		}
		parts = append(parts, fmt.Sprintf("(%s)", year))
	}

	if rg.PrimaryType != "" {
		parts = append(parts, typeStyle.Render(fmt.Sprintf("[%s]", rg.PrimaryType)))
	}

	if len(rg.SecondaryTypes) > 0 {
		parts = append(parts, dimStyle.Render("+"+strings.Join(rg.SecondaryTypes, "+")))
	}

	return strings.Join(parts, " ")
}
