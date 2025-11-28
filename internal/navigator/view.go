package navigator

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func (m Model[T]) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	path := m.source.DisplayPath(m.current)
	header := runewidth.Truncate(path, m.width, "...")
	header = runewidth.FillRight(header, m.width)
	separator := strings.Repeat("─", m.width)

	listHeight := m.height - 4
	col1Width := m.width / 6
	col2Width := m.width / 6
	col3Width := m.width - col1Width - col2Width - 2

	var parentCol []string
	if m.source.Parent(m.current) == nil {
		parentCol = m.renderEmptyColumn(col1Width, listHeight)
	} else {
		parentCol = m.renderColumn(m.parentItems, -1, 0, col1Width, listHeight)
	}
	currentCol := m.renderColumn(m.currentItems, m.cursor, m.offset, col2Width, listHeight)
	previewCol := m.renderColumn(m.previewItems, -1, 0, col3Width, listHeight)

	result := header + "\n" + separator + "\n" + m.joinColumns(parentCol, currentCol, previewCol)

	// Overlay selected item name with highlight style
	if selected := m.Selected(); selected != nil {
		name := (*selected).DisplayName()
		if (*selected).IsContainer() {
			name += "/"
		}
		styledOverlay := "> " + selectionStyle.Render(name)
		// Overlay from col2 start, stopping before second separator
		result = m.overlayBox(result, styledOverlay, col1Width+1, m.cursor-m.offset+2, col1Width+col2Width+1)
	}

	return result
}

func (m Model[T]) overlayBox(base, box string, x, y, maxX int) string {
	baseLines := strings.Split(base, "\n")
	boxLines := strings.Split(box, "\n")

	for i, boxLine := range boxLines {
		targetY := y + i
		if targetY < 0 || targetY >= len(baseLines) {
			continue
		}
		baseLines[targetY] = m.overlayLine(baseLines[targetY], boxLine, x, maxX)
	}

	return strings.Join(baseLines, "\n")
}

func (m Model[T]) overlayLine(baseLine, overlay string, x, _ int) string {
	overlayWidth := lipgloss.Width(overlay)
	endX := x + overlayWidth

	var result strings.Builder
	pos := 0
	overlayWritten := false

	for _, r := range baseLine {
		w := runewidth.RuneWidth(r)
		if pos >= x && pos < endX {
			if !overlayWritten {
				result.WriteString(overlay)
				overlayWritten = true
			}
		} else {
			result.WriteRune(r)
		}
		pos += w
	}

	return result.String()
}

func (m Model[T]) renderEmptyColumn(width, height int) []string {
	lines := make([]string, height)
	for i := range height {
		lines[i] = strings.Repeat(" ", width)
	}
	return lines
}

func (m Model[T]) renderColumn(
	items []T,
	cursor int,
	offset int,
	width int,
	height int,
) []string {
	lines := make([]string, height)

	for i := range height {
		idx := i + offset
		if idx < len(items) {
			node := items[idx]
			name := node.DisplayName()
			if node.IsContainer() {
				name += "/"
			}

			name = runewidth.Truncate(name, width-2, "...")

			prefix := "  "
			if idx == cursor {
				prefix = "> "
			}

			line := prefix + name
			line = runewidth.FillRight(line, width)
			lines[i] = line
		} else {
			lines[i] = strings.Repeat(" ", width)
		}
	}

	return lines
}

func (m Model[T]) joinColumns(col1, col2, col3 []string) string {
	var sb strings.Builder

	maxLen := max(len(col1), len(col2), len(col3))
	for i := range maxLen {
		if i < len(col1) {
			sb.WriteString(col1[i])
		}
		sb.WriteString("│")
		if i < len(col2) {
			sb.WriteString(col2[i])
		}
		sb.WriteString("│")
		if i < len(col3) {
			sb.WriteString(col3[i])
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
