package navigator

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

func (m Model[T]) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Account for border (2 chars each side)
	innerWidth := m.width - ui.BorderHeight
	// Account for border + header + separator
	listHeight := m.height - ui.PanelOverhead

	path := m.source.DisplayPath(m.current)
	header := render.TruncateAndPad(path, innerWidth)
	separator := render.Separator(innerWidth)

	// 3-column layout: parent (20%) | current (40%) | preview (40%)
	// Account for 2 separators (│)
	availableWidth := innerWidth - 2
	parentColWidth := availableWidth / 5        // 20%
	currentColWidth := (availableWidth * 2) / 5 // 40%
	previewColWidth := availableWidth - parentColWidth - currentColWidth

	// Calculate parent offset to center the parent cursor
	parentOffset := m.calculateParentOffset(listHeight)

	parentCol := m.renderColumn(m.parentItems, m.parentCursor, parentOffset, parentColWidth, listHeight)
	currentCol := m.renderColumn(m.currentItems, m.cursor, m.offset, currentColWidth, listHeight)
	previewCol := m.renderColumn(m.previewItems, -1, 0, previewColWidth, listHeight)

	content := header + "\n" + separator + "\n" + m.joinThreeColumns(parentCol, currentCol, previewCol)

	// Overlay selected item name with highlight style (only when focused)
	// The overlay goes in the middle column (after parent column + separator)
	if m.focused {
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
			case IconPlaylist:
				name = icons.FormatPlaylist(name)
			}
			styledOverlay := "> " + selectionStyle.Render(name)
			// Overlay starts after parent column + separator
			overlayX := parentColWidth + 1
			content = m.overlayBox(content, styledOverlay, overlayX, m.cursor-m.offset+2, currentColWidth)
		}
	}

	return styles.PanelStyle(m.focused).Width(innerWidth).Render(content)
}

func (m Model[T]) calculateParentOffset(listHeight int) int {
	if m.parentCursor < 0 || len(m.parentItems) == 0 {
		return 0
	}

	// Center the parent cursor in the column
	offset := max(0, m.parentCursor-listHeight/2)
	maxOffset := max(0, len(m.parentItems)-listHeight)
	return min(offset, maxOffset)
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
			case IconPlaylist:
				name = icons.FormatPlaylist(name)
			}

			name = render.Truncate(name, width-2)

			prefix := "  "
			if idx == cursor {
				prefix = "> "
			}

			line := prefix + name
			lines[i] = render.Pad(line, width)
		} else {
			lines[i] = render.EmptyLine(width)
		}
	}

	return lines
}

func (m Model[T]) joinThreeColumns(col1, col2, col3 []string) string {
	maxLen := max(len(col1), len(col2), len(col3))
	lines := make([]string, maxLen)

	for i := range maxLen {
		var sb strings.Builder
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
		lines[i] = sb.String()
	}

	return strings.Join(lines, "\n")
}
