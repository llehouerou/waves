package navigator

import (
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// columnType represents the type of column being rendered.
type columnType int

const (
	columnParent columnType = iota
	columnCurrent
	columnPreview
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
	header := headerStyle().Render(render.TruncateAndPad(path, innerWidth))
	separator := render.Separator(innerWidth)

	// 3-column layout: parent (20%) | current (40%) | preview (40%)
	// Account for 2 separators (│)
	availableWidth := innerWidth - 2
	parentColWidth := availableWidth / 5        // 20%
	currentColWidth := (availableWidth * 2) / 5 // 40%
	previewColWidth := availableWidth - parentColWidth - currentColWidth

	// Calculate parent offset to center the parent cursor
	parentOffset := m.calculateParentOffset(listHeight)

	parentCol := m.renderColumn(m.parentItems, m.parentCursor, parentOffset, parentColWidth, listHeight, columnParent)
	currentCol := m.renderColumn(m.currentItems, m.cursor.Pos(), m.cursor.Offset(), currentColWidth, listHeight, columnCurrent)

	var previewCol []string
	if m.previewLines != nil {
		previewCol = m.renderPreviewLines(m.previewLines, previewColWidth, listHeight)
	} else {
		previewCol = m.renderColumn(m.previewItems, -1, 0, previewColWidth, listHeight, columnPreview)
	}

	content := header + "\n" + separator + "\n" + m.joinThreeColumns(parentCol, currentCol, previewCol)

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

func (m Model[T]) renderColumn(
	items []T,
	cursor int,
	offset int,
	width int,
	height int,
	colType columnType,
) []string {
	lines := make([]string, height)

	for i := range height {
		idx := i + offset
		if idx >= len(items) {
			lines[i] = render.EmptyLine(width)
			continue
		}
		lines[i] = m.renderColumnItem(items[idx], idx, cursor, width, colType)
	}

	return lines
}

func (m Model[T]) renderColumnItem(node T, idx, cursor, width int, colType columnType) string {
	name := formatNodeName(node)
	isFavorite := m.isNodeFavorite(node)

	favIcon := icons.Favorite()
	favIconWidth := runewidth.StringWidth(favIcon)

	// Reserve space for favorite icon if needed
	maxNameWidth := width - 2 // 2 for prefix
	if isFavorite {
		maxNameWidth -= favIconWidth + 1 // +1 for space before icon
	}
	name = render.Truncate(name, maxNameWidth)

	prefix := "  "
	if idx == cursor {
		prefix = "> "
	}

	line := prefix + name

	// Build the full line with padding and optional favorite icon
	var fullLine string
	if !isFavorite {
		fullLine = render.Pad(line, width)
	} else {
		// Right-align the favorite icon
		currentWidth := runewidth.StringWidth(line)
		padding := width - currentWidth - favIconWidth
		if padding > 0 {
			fullLine = line + strings.Repeat(" ", padding) + favIcon
		} else {
			fullLine = render.Pad(line, width-favIconWidth) + favIcon
		}
	}

	// Apply styling based on column type and cursor state
	return m.styleColumnItem(fullLine, idx, cursor, colType)
}

func (m Model[T]) styleColumnItem(line string, idx, cursor int, colType columnType) string {
	isCursor := idx == cursor && m.focused

	switch colType {
	case columnCurrent:
		if isCursor {
			return cursorStyle().Render(line)
		}
		return currentColumnStyle().Render(line)
	case columnParent, columnPreview:
		return sideColumnStyle().Render(line)
	}
	return line
}

func formatNodeName[T Node](node T) string {
	name := node.DisplayName()
	switch node.IconType() {
	case IconArtist:
		return icons.FormatArtist(name)
	case IconAlbum:
		return icons.FormatAlbum(name)
	case IconFolder:
		return icons.FormatDir(name)
	case IconAudio:
		return icons.FormatAudio(name)
	case IconPlaylist:
		return icons.FormatPlaylist(name)
	}
	return name
}

func (m Model[T]) isNodeFavorite(node T) bool {
	provider, ok := any(node).(TrackIDProvider)
	if !ok {
		return false
	}
	trackID := provider.TrackID()
	return trackID != 0 && m.IsFavorite(trackID)
}

func (m Model[T]) renderPreviewLines(lines []string, width, height int) []string {
	result := make([]string, height)
	style := sideColumnStyle()

	for i := range height {
		if i < len(lines) {
			line := render.TruncateAndPad(lines[i], width)
			result[i] = style.Render(line)
		} else {
			result[i] = render.EmptyLine(width)
		}
	}

	return result
}

func (m Model[T]) joinThreeColumns(col1, col2, col3 []string) string {
	maxLen := max(len(col1), len(col2), len(col3))
	lines := make([]string, maxLen)
	sep := columnSeparatorStyle().Render("│")

	for i := range maxLen {
		var sb strings.Builder
		if i < len(col1) {
			sb.WriteString(col1[i])
		}
		sb.WriteString(sep)
		if i < len(col2) {
			sb.WriteString(col2[i])
		}
		sb.WriteString(sep)
		if i < len(col3) {
			sb.WriteString(col3[i])
		}
		lines[i] = sb.String()
	}

	return strings.Join(lines, "\n")
}
