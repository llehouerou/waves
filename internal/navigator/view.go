package navigator

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/llehouerou/waves/internal/icons"
)

func (m Model[T]) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Account for border (2 chars each side)
	innerWidth := m.width - 2
	// Account for border (2 lines) + header (1) + separator (1) + trailing newline (1)
	listHeight := m.height - 5

	path := m.source.DisplayPath(m.current)
	header := runewidth.Truncate(path, innerWidth, "...")
	header = runewidth.FillRight(header, innerWidth)
	separator := strings.Repeat("─", innerWidth)

	currentWidth := innerWidth / 4
	previewWidth := innerWidth - currentWidth - 1

	currentCol := m.renderColumn(m.currentItems, m.cursor, m.offset, currentWidth, listHeight)
	previewCol := m.renderColumn(m.previewItems, -1, 0, previewWidth, listHeight)

	content := header + "\n" + separator + "\n" + m.joinColumns(currentCol, previewCol)

	// Overlay selected item name with highlight style
	if selected := m.Selected(); selected != nil {
		name := (*selected).DisplayName()
		switch (*selected).IconType() {
		case IconArtist:
			name = icons.FormatArtist(name)
		case IconAlbum:
			name = icons.FormatAlbum(name)
		case IconFolder:
			name = icons.FormatDir(name)
		case IconAudio:
			name = icons.FormatAudio(name)
		}
		styledOverlay := "> " + selectionStyle.Render(name)
		content = m.overlayBox(content, styledOverlay, 0, m.cursor-m.offset+2, currentWidth)
	}

	return panelStyle.Width(innerWidth).Render(content)
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
			switch node.IconType() {
			case IconArtist:
				name = icons.FormatArtist(name)
			case IconAlbum:
				name = icons.FormatAlbum(name)
			case IconFolder:
				name = icons.FormatDir(name)
			case IconAudio:
				name = icons.FormatAudio(name)
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

func (m Model[T]) joinColumns(col1, col2 []string) string {
	var sb strings.Builder

	maxLen := max(len(col1), len(col2))
	for i := range maxLen {
		if i < len(col1) {
			sb.WriteString(col1[i])
		}
		sb.WriteString("│")
		if i < len(col2) {
			sb.WriteString(col2[i])
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
