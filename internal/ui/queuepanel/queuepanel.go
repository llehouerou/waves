package queuepanel

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// JumpToTrackMsg is sent when the user selects a track to jump to.
type JumpToTrackMsg struct {
	Index int
}

// QueueChangedMsg is sent when the queue is modified (delete, move).
type QueueChangedMsg struct{}

// Model represents the queue panel state.
type Model struct {
	queue    *playlist.PlayingQueue
	cursor   int
	offset   int
	width    int
	height   int
	focused  bool
	selected map[int]bool
}

// New creates a new queue panel model.
func New(queue *playlist.PlayingQueue) Model {
	return Model{
		queue:    queue,
		cursor:   0,
		offset:   0,
		selected: make(map[int]bool),
	}
}

// SetFocused sets whether the panel is focused.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether the panel is focused.
func (m Model) IsFocused() bool {
	return m.focused
}

// SetSize sets the panel dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages for the queue panel.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if !m.focused {
		return m, nil
	}

	switch keyMsg.String() {
	case "x":
		// Toggle selection on current item
		if m.queue.Len() > 0 && m.cursor < m.queue.Len() {
			if m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = true
			}
		}
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "g":
		m.cursor = 0
		m.offset = 0
	case "G":
		if m.queue.Len() > 0 {
			m.cursor = m.queue.Len() - 1
			m.ensureCursorVisible()
		}
	case "enter":
		if m.queue.Len() > 0 && m.cursor < m.queue.Len() {
			m.clearSelection()
			return m, func() tea.Msg {
				return JumpToTrackMsg{Index: m.cursor}
			}
		}
	case "d", "delete":
		if m.queue.Len() > 0 {
			m.deleteSelected()
			return m, func() tea.Msg { return QueueChangedMsg{} }
		}
	case "esc":
		if len(m.selected) > 0 {
			m.clearSelection()
		}
	case "shift+j", "shift+down":
		if m.moveSelected(1) {
			return m, func() tea.Msg { return QueueChangedMsg{} }
		}
	case "shift+k", "shift+up":
		if m.moveSelected(-1) {
			return m, func() tea.Msg { return QueueChangedMsg{} }
		}
	}

	return m, nil
}

// SyncCursor moves the cursor to the currently playing track.
func (m *Model) SyncCursor() {
	currentIdx := m.queue.CurrentIndex()
	if currentIdx >= 0 && currentIdx < m.queue.Len() {
		m.cursor = currentIdx
		m.ensureCursorVisible()
	}
}

func (m *Model) moveCursor(delta int) {
	if m.queue.Len() == 0 {
		return
	}

	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= m.queue.Len() {
		m.cursor = m.queue.Len() - 1
	}

	m.ensureCursorVisible()
}

func (m *Model) ensureCursorVisible() {
	listHeight := m.listHeight()
	if listHeight <= 0 {
		return
	}

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+listHeight {
		m.offset = m.cursor - listHeight + 1
	}
}

func (m *Model) clearSelection() {
	m.selected = make(map[int]bool)
}

func (m *Model) moveSelected(delta int) bool {
	if m.queue.Len() == 0 {
		return false
	}

	// Get indices to move (selected or cursor)
	var indices []int
	if len(m.selected) > 0 {
		indices = make([]int, 0, len(m.selected))
		for idx := range m.selected {
			indices = append(indices, idx)
		}
	} else {
		indices = []int{m.cursor}
	}

	// Perform the move
	newIndices, moved := m.queue.MoveIndices(indices, delta)
	if !moved {
		return false
	}

	// Update selection with new indices
	if len(m.selected) > 0 {
		m.selected = make(map[int]bool)
		for _, idx := range newIndices {
			m.selected[idx] = true
		}
	}

	// Move cursor along with the selection
	m.cursor += delta
	m.ensureCursorVisible()
	return true
}

func (m *Model) deleteSelected() {
	// If we have a selection, delete selected items
	// Otherwise delete just the cursor item
	if len(m.selected) == 0 {
		m.selected[m.cursor] = true
	}

	// Get sorted indices in descending order to delete from end first
	indices := make([]int, 0, len(m.selected))
	for idx := range m.selected {
		indices = append(indices, idx)
	}
	// Sort descending
	for i := range indices {
		for j := i + 1; j < len(indices); j++ {
			if indices[j] > indices[i] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	// Delete from highest index first
	for _, idx := range indices {
		m.queue.RemoveAt(idx)
	}

	// Clear selection
	m.selected = make(map[int]bool)

	// Adjust cursor if it's now past the end
	if m.cursor >= m.queue.Len() && m.cursor > 0 {
		m.cursor = m.queue.Len() - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

func (m Model) listHeight() int {
	// Account for border + header + separator
	return m.height - ui.PanelOverhead
}

// View renders the queue panel.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	innerWidth := m.width - ui.BorderHeight // border padding
	listHeight := m.listHeight()

	// Header with mode icons on the right
	var headerLeftText string
	var headerStyle lipgloss.Style
	if len(m.selected) > 0 {
		headerLeftText = fmt.Sprintf("Queue [%d selected]", len(m.selected))
		headerStyle = multiSelectHeaderStyle
	} else {
		currentIdx := m.queue.CurrentIndex() + 1
		if currentIdx < 1 {
			currentIdx = 0
		}
		headerLeftText = fmt.Sprintf("Queue (%d/%d)", currentIdx, m.queue.Len())
		headerStyle = defaultHeaderStyle
	}

	// Mode icons on the right
	modeIcons, modeIconsWidth := m.renderModeIcons()

	// Calculate available width for header text (truncate/pad raw text, then style)
	headerLeftWidth := innerWidth - modeIconsWidth
	headerLeftText = render.TruncateAndPad(headerLeftText, headerLeftWidth)

	header := headerStyle.Render(headerLeftText) + modeIcons

	// Separator
	separator := render.Separator(innerWidth)

	// Track list
	tracks := m.queue.Tracks()
	playingIdx := m.queue.CurrentIndex()

	lines := make([]string, 0, listHeight)
	for i := range listHeight {
		idx := i + m.offset
		if idx >= len(tracks) {
			lines = append(lines, render.EmptyLine(innerWidth))
			continue
		}

		track := tracks[idx]
		line := m.renderTrackLine(track, idx, playingIdx, innerWidth)
		lines = append(lines, line)
	}

	content := header + "\n" + separator + "\n" + strings.Join(lines, "\n")

	return styles.PanelStyle(m.focused).
		Width(innerWidth).
		Render(content)
}

// renderModeIcons returns the styled mode icons and their display width.
func (m Model) renderModeIcons() (styled string, width int) {
	var parts []string

	if m.queue.Shuffle() {
		parts = append(parts, icons.Shuffle())
	}

	switch m.queue.RepeatMode() {
	case playlist.RepeatOff:
		// No icon for repeat off
	case playlist.RepeatAll:
		parts = append(parts, icons.RepeatAll())
	case playlist.RepeatOne:
		parts = append(parts, icons.RepeatOne())
	}

	if len(parts) == 0 {
		return "", 0
	}

	// Join with double space for better separation
	raw := strings.Join(parts, "  ")
	// Icons are 1 cell wide each, plus 2 spaces between, plus 1 space padding from border
	width = len(parts) + (len(parts)-1)*2 + 1
	styled = modeIconStyle.Render(raw) + " "
	return styled, width
}

func (m Model) renderTrackLine(track playlist.Track, idx, playingIdx, width int) string {
	// Prefix: "▶ " for playing, "  " otherwise
	prefix := "  "
	if idx == playingIdx {
		prefix = playingSymbol + " "
	}

	// Always reserve space for selection marker
	suffixWidth := 2 // " ●"
	suffix := "  "
	if m.selected[idx] {
		suffix = " " + selectedSymbol
	}

	// Calculate available width for content
	prefixWidth := 2
	contentWidth := width - prefixWidth - suffixWidth

	// Two-column layout: title on left (half), artist on right (half)
	title := track.Title
	artist := track.Artist

	colWidth := contentWidth / 2
	titleWidth := colWidth
	artistWidth := contentWidth - titleWidth

	title = render.TruncateAndPad(title, titleWidth)
	artist = render.TruncateAndPad(artist, artistWidth)

	line := prefix + title + artist + suffix

	// Apply styling based on track state
	style := m.trackStyle(idx, playingIdx)

	return style.Render(line)
}

func (m Model) trackStyle(idx, playingIdx int) lipgloss.Style {
	isCursor := idx == m.cursor && m.focused
	isPlaying := idx == playingIdx
	isPlayed := idx < playingIdx

	switch {
	case isCursor && isPlaying:
		return cursorStyle.Inherit(playingStyle)
	case isCursor && isPlayed:
		return cursorStyle.Inherit(dimmedStyle)
	case isCursor:
		return cursorStyle
	case isPlaying:
		return playingStyle
	case isPlayed:
		return dimmedStyle
	default:
		return trackStyle
	}
}
