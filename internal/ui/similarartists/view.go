package similarartists

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

func titleStyle() lipgloss.Style {
	return styles.T().BaseStyle().
		Bold(true).
		Foreground(styles.T().Primary).
		MarginBottom(1)
}

func sectionStyle() lipgloss.Style {
	return styles.T().BaseStyle().
		Foreground(styles.T().FgMuted).
		Bold(true)
}

func selectedStyle() lipgloss.Style {
	return styles.T().BaseStyle().
		Foreground(styles.T().Primary).
		Bold(true)
}

func normalStyle() lipgloss.Style {
	return styles.T().BaseStyle().
		Foreground(styles.T().FgBase)
}

func scoreStyle() lipgloss.Style {
	return styles.T().BaseStyle().
		Foreground(styles.T().FgMuted)
}

func helpStyle() lipgloss.Style {
	return styles.T().BaseStyle().
		Foreground(styles.T().FgMuted).
		MarginTop(1)
}

func errorStyle() lipgloss.Style {
	return styles.T().BaseStyle().
		Foreground(styles.T().Error)
}

// View renders the popup content.
func (m *Model) View() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle().Render("Similar to: " + m.artistName))
	b.WriteString("\n")

	// Loading state
	if m.loading {
		b.WriteString("\nLoading...")
		return b.String()
	}

	// Error state
	if m.errorMsg != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle().Render(m.errorMsg))
		b.WriteString("\n\n")
		b.WriteString(helpStyle().Render("esc: close"))
		return b.String()
	}

	// Empty state
	if m.totalItems() == 0 {
		b.WriteString("\nNo similar artists found.")
		b.WriteString("\n\n")
		b.WriteString(helpStyle().Render("esc: close"))
		return b.String()
	}

	// In Library section
	if len(m.inLibrary) > 0 {
		b.WriteString("\n")
		b.WriteString(sectionStyle().Render("In Library"))
		b.WriteString("\n")
		b.WriteString(render.Separator(40))
		b.WriteString("\n")
		for i, item := range m.inLibrary {
			b.WriteString(m.renderItem(item, i))
			b.WriteString("\n")
		}
	}

	// Not in Library section
	if len(m.notInLibrary) > 0 {
		b.WriteString("\n")
		b.WriteString(sectionStyle().Render("Not in Library"))
		b.WriteString("\n")
		b.WriteString(render.Separator(40))
		b.WriteString("\n")
		for i, item := range m.notInLibrary {
			idx := len(m.inLibrary) + i
			b.WriteString(m.renderItem(item, idx))
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString("\n")
	b.WriteString(helpStyle().Render("enter: go/download  d: download  esc: close"))

	return b.String()
}

func (m *Model) renderItem(item SimilarArtistItem, index int) string {
	cursor := "  "
	style := normalStyle
	if index == m.cursor {
		cursor = "> "
		style = selectedStyle
	}

	score := scoreStyle().Render(fmt.Sprintf("(%d%%)", int(item.MatchScore*100)))
	return fmt.Sprintf("%s%s %s", cursor, style().Render(item.Name), score)
}
