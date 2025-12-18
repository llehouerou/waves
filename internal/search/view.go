package search

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/styles"
)

const maxVisibleResults = 20

func popupStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.T().Border)
}

func inputStyle() lipgloss.Style {
	return styles.T().S().Base
}

func selectedStyle() lipgloss.Style {
	return styles.T().S().Playing
}

func normalStyle() lipgloss.Style {
	return styles.T().S().Base
}

func dimStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func (m Model) popupWidth() int {
	w := m.width * 60 / 100
	if w < 40 {
		w = min(40, m.width-4)
	}
	return w
}

func (m Model) popupHeight() int {
	h := m.height * 50 / 100
	if h < 10 {
		h = min(10, m.height-2)
	}
	return h
}

func (m Model) visibleHeight() int {
	// Account for border (2) + input line (1) + separator (1)
	h := max(m.popupHeight()-4, 1)
	return min(h, maxVisibleResults)
}

func (m Model) emptyMessage() string {
	switch {
	case m.loading:
		return "Scanning..."
	case m.query != "":
		return "No matches"
	default:
		return "Type to search..."
	}
}

func (m Model) formatResultLine(item Item, innerW int, isCursor bool) string {
	prefix := "  "
	if isCursor {
		prefix = "> "
	}

	// Check if item supports two-column display
	twoCol, ok := item.(TwoColumnItem)
	if !ok {
		// Fallback to single column display
		text := item.DisplayText()
		if lipgloss.Width(text) > innerW-4 {
			text = text[:innerW-7] + "..."
		}
		return prefix + text
	}

	left := twoCol.LeftColumn()
	right := twoCol.RightColumn()
	availW := innerW - 4 // account for prefix and padding

	if right == "" {
		// No right column, just show left
		if lipgloss.Width(left) > availW {
			left = left[:availW-3] + "..."
		}
		return prefix + left
	}

	// Truncate left column if needed, leaving space for right
	rightW := lipgloss.Width(right)
	maxLeftW := availW - rightW - 2 // 2 for gap
	if lipgloss.Width(left) > maxLeftW {
		if maxLeftW > 3 {
			left = left[:maxLeftW-3] + "..."
		} else if maxLeftW > 0 {
			left = left[:maxLeftW]
		}
	}

	// Build line with left-aligned name and right-aligned path
	gap := max(1, availW-lipgloss.Width(left)-rightW)
	return prefix + left + strings.Repeat(" ", gap) + dimStyle().Render(right)
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	popupW := m.popupWidth()
	innerW := popupW - 2 // account for border

	// Input line
	prompt := "> "
	input := inputStyle().Render(prompt + m.query)

	// Separator
	separator := strings.Repeat("─", innerW)

	// Results
	visible := m.visibleHeight()
	var resultLines []string

	if len(m.matches) == 0 {
		resultLines = append(resultLines, dimStyle().Render(m.emptyMessage()))
	} else {
		end := min(m.offset+visible, len(m.matches))
		for i := m.offset; i < end; i++ {
			match := m.matches[i]
			item := m.items[match.Index]
			isCursor := i == m.cursor
			line := m.formatResultLine(item, innerW, isCursor)

			if isCursor {
				resultLines = append(resultLines, selectedStyle().Render(line))
			} else {
				resultLines = append(resultLines, normalStyle().Render(line))
			}
		}
	}

	// Loading indicator in input line
	inputLine := input
	if m.loading {
		spinnerChar := "◐" // simple spinner
		inputLine = input + dimStyle().Render(" "+spinnerChar)
	}

	// Pad result lines to fill popup height
	for len(resultLines) < visible {
		resultLines = append(resultLines, "")
	}

	// Build popup content
	content := inputLine + "\n" + separator + "\n" + strings.Join(resultLines, "\n")

	// Style the popup with border
	box := popupStyle().Width(innerW).Render(content)

	// Center in terminal
	return popup.Center(box, m.width, m.height)
}
