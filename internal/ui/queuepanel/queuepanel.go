package queuepanel

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// JumpToTrackMsg is sent when the user selects a track to jump to.
type JumpToTrackMsg struct {
	Index int
}

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
		}
	case "esc":
		if len(m.selected) > 0 {
			m.clearSelection()
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
	// Account for border (2 lines) + header (1 line) + separator (1 line)
	return m.height - 4
}

// View renders the queue panel.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	innerWidth := m.width - 2 // border padding
	listHeight := m.listHeight()

	// Header
	var header string
	if len(m.selected) > 0 {
		header = fmt.Sprintf("Queue [%d selected]", len(m.selected))
		header = multiSelectHeaderStyle.Render(header)
	} else {
		currentIdx := m.queue.CurrentIndex() + 1
		if currentIdx < 1 {
			currentIdx = 0
		}
		header = fmt.Sprintf("Queue (%d/%d)", currentIdx, m.queue.Len())
		header = headerStyle.Render(header)
	}
	header = runewidth.Truncate(header, innerWidth, "...")
	header = runewidth.FillRight(header, innerWidth)

	// Separator
	separator := strings.Repeat("─", innerWidth)

	// Track list
	tracks := m.queue.Tracks()
	playingIdx := m.queue.CurrentIndex()

	lines := make([]string, 0, listHeight)
	for i := range listHeight {
		idx := i + m.offset
		if idx >= len(tracks) {
			lines = append(lines, strings.Repeat(" ", innerWidth))
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

func (m Model) renderTrackLine(track playlist.Track, idx, playingIdx, width int) string {
	// Build the display string: "▶ Title - Artist" or "  Title - Artist"
	prefix := "  "
	if idx == playingIdx {
		prefix = playingSymbol + " "
	}

	// Suffix for selected items
	suffix := ""
	isSelected := m.selected[idx]
	if isSelected {
		suffix = " " + selectedSymbol
	}

	// Format track info
	info := track.Title
	if track.Artist != "" {
		info += " - " + track.Artist
	}

	// Truncate to fit (account for prefix and suffix)
	maxInfoWidth := width - 2 - runewidth.StringWidth(suffix) // prefix width + suffix
	info = runewidth.Truncate(info, maxInfoWidth, "...")

	line := prefix + info
	line = runewidth.FillRight(line, width-runewidth.StringWidth(suffix))
	line += suffix

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
