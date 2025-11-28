package navigator

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var overlayStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("236")).
	Foreground(lipgloss.Color("252")).
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("240")).
	BorderTop(false).
	BorderBottom(false)

// NavigationChangedMsg is sent when the current folder or selection changes.
type NavigationChangedMsg struct {
	CurrentPath  string
	SelectedName string
}

type Model[T Node] struct {
	source       Source[T]
	current      T
	parentItems  []T
	currentItems []T
	previewItems []T
	cursor       int
	offset       int
	width        int
	height       int
}

func New[T Node](source Source[T]) (Model[T], error) {
	m := Model[T]{
		source:  source,
		current: source.Root(),
	}

	if err := m.refresh(); err != nil {
		return Model[T]{}, err
	}

	return m, nil
}

func (m *Model[T]) refresh() error {
	var err error

	parent := m.source.Parent(m.current)
	if parent != nil {
		m.parentItems, err = m.source.Children(*parent)
		if err != nil {
			m.parentItems = nil
		}
	} else {
		m.parentItems = nil
	}

	m.currentItems, err = m.source.Children(m.current)
	if err != nil {
		return err
	}

	if m.cursor >= len(m.currentItems) {
		m.cursor = max(0, len(m.currentItems)-1)
	}

	m.updatePreview()
	return nil
}

func (m *Model[T]) updatePreview() {
	if len(m.currentItems) == 0 || m.cursor >= len(m.currentItems) {
		m.previewItems = nil
		return
	}

	selected := m.currentItems[m.cursor]
	if selected.IsContainer() {
		items, err := m.source.Children(selected)
		if err != nil {
			m.previewItems = nil
			return
		}
		m.previewItems = items
	} else {
		m.previewItems = nil
	}
}

func (m *Model[T]) adjustOffset() {
	listHeight := m.height - 4
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

func (m *Model[T]) focusNode(id string) {
	for i, node := range m.currentItems {
		if node.ID() == id {
			m.cursor = i
			m.centerCursor()
			m.updatePreview()
			return
		}
	}
	m.cursor = 0
	m.offset = 0
	m.updatePreview()
}

// FocusByName selects the item with the given display name.
// If not found, selection stays at current position.
func (m *Model[T]) FocusByName(name string) {
	for i, node := range m.currentItems {
		if node.DisplayName() == name {
			m.cursor = i
			m.centerCursor()
			m.updatePreview()
			return
		}
	}
}

// NavigateTo navigates to the given node ID (for files, navigates to parent and selects).
func (m *Model[T]) NavigateTo(id string) bool {
	node, ok := m.source.NodeFromID(id)
	if !ok {
		return false
	}

	if node.IsContainer() {
		// Navigate into the directory
		m.current = node
		m.cursor = 0
		m.offset = 0
		_ = m.refresh()
	} else {
		// Navigate to parent directory and select the file
		parent := m.source.Parent(node)
		if parent == nil {
			return false
		}
		m.current = *parent
		m.cursor = 0
		m.offset = 0
		_ = m.refresh()
		m.FocusByName(node.DisplayName())
	}

	return true
}

func (m *Model[T]) centerCursor() {
	listHeight := m.height - 4
	if listHeight <= 0 {
		return
	}

	m.offset = m.cursor - listHeight/2
	if m.offset < 0 {
		m.offset = 0
	}

	maxOffset := len(m.currentItems) - listHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}

func (m Model[T]) Selected() *T {
	if len(m.currentItems) == 0 || m.cursor >= len(m.currentItems) {
		return nil
	}
	return &m.currentItems[m.cursor]
}

// CurrentPath returns the display path of the current folder.
func (m Model[T]) CurrentPath() string {
	return m.source.DisplayPath(m.current)
}

// SelectedName returns the display name of the selected item, or empty if none.
func (m Model[T]) SelectedName() string {
	if selected := m.Selected(); selected != nil {
		return (*selected).DisplayName()
	}
	return ""
}

func (m Model[T]) navigationChangedCmd() tea.Cmd {
	return func() tea.Msg {
		return NavigationChangedMsg{
			CurrentPath:  m.CurrentPath(),
			SelectedName: m.SelectedName(),
		}
	}
}

func (m Model[T]) Init() tea.Cmd {
	return nil
}

func (m Model[T]) Update(msg tea.Msg) (Model[T], tea.Cmd) {
	var navChanged bool

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.adjustOffset()
				m.updatePreview()
				navChanged = true
			}

		case "down", "j":
			if m.cursor < len(m.currentItems)-1 {
				m.cursor++
				m.adjustOffset()
				m.updatePreview()
				navChanged = true
			}

		case "left", "h":
			parent := m.source.Parent(m.current)
			if parent != nil {
				prevID := m.current.ID()
				m.current = *parent
				_ = m.refresh()
				m.focusNode(prevID)
				navChanged = true
			}

		case "right", "l", "enter":
			if len(m.currentItems) > 0 {
				selected := m.currentItems[m.cursor]
				if selected.IsContainer() {
					m.current = selected
					m.cursor = 0
					m.offset = 0
					_ = m.refresh()
					navChanged = true
				}
			}
		}
	}

	if navChanged {
		return m, m.navigationChangedCmd()
	}
	return m, nil
}

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
		styledOverlay := "> " + overlayStyle.Render(name)
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
