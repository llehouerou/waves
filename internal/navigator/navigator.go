package navigator

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

type Model struct {
	currentPath  string
	parentPath   string
	parentItems  []Entry
	currentItems []Entry
	previewItems []Entry
	cursor       int
	offset       int
	width        int
	height       int
}

func New(startPath string) (Model, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return Model{}, err
	}

	m := Model{
		currentPath: absPath,
		parentPath:  filepath.Dir(absPath),
	}

	if err := m.refresh(); err != nil {
		return Model{}, err
	}

	return m, nil
}

func (m *Model) refresh() error {
	var err error

	m.parentItems, err = ListDir(m.parentPath)
	if err != nil {
		m.parentItems = nil
	}

	m.currentItems, err = ListDir(m.currentPath)
	if err != nil {
		return err
	}

	if m.cursor >= len(m.currentItems) {
		m.cursor = max(0, len(m.currentItems)-1)
	}

	m.updatePreview()
	return nil
}

func (m *Model) updatePreview() {
	if len(m.currentItems) == 0 || m.cursor >= len(m.currentItems) {
		m.previewItems = nil
		return
	}

	selected := m.currentItems[m.cursor]
	if selected.IsDir {
		items, err := ListDir(selected.Path)
		if err != nil {
			m.previewItems = nil
			return
		}
		m.previewItems = items
	} else {
		m.previewItems = nil
	}
}

func (m *Model) adjustOffset() {
	listHeight := m.height - 2
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

func (m *Model) focusEntry(name string) {
	for i, entry := range m.currentItems {
		if entry.Name == name {
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

func (m *Model) centerCursor() {
	listHeight := m.height - 2
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

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
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
			}

		case "down", "j":
			if m.cursor < len(m.currentItems)-1 {
				m.cursor++
				m.adjustOffset()
				m.updatePreview()
			}

		case "left", "h":
			if m.parentPath != m.currentPath {
				prevDir := filepath.Base(m.currentPath)
				m.currentPath = m.parentPath
				m.parentPath = filepath.Dir(m.parentPath)
				_ = m.refresh()
				m.focusEntry(prevDir)
			}

		case "right", "l", "enter":
			if len(m.currentItems) > 0 {
				selected := m.currentItems[m.cursor]
				if selected.IsDir {
					m.parentPath = m.currentPath
					m.currentPath = selected.Path
					m.cursor = 0
					m.offset = 0
					_ = m.refresh()
				}
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	header := runewidth.Truncate(m.currentPath, m.width, "...")
	header = runewidth.FillRight(header, m.width)
	separator := strings.Repeat("─", m.width)

	listHeight := m.height - 4 // -1 for header, -1 for separator, -2 for padding
	col1Width := m.width / 6
	col2Width := m.width / 6
	col3Width := m.width - col1Width - col2Width - 2 // -2 for separators

	var parentCol []string
	if m.currentPath == "/" {
		parentCol = m.renderEmptyColumn(col1Width, listHeight)
	} else {
		parentCol = m.renderColumn(m.parentItems, -1, 0, col1Width, listHeight)
	}
	currentCol := m.renderColumn(m.currentItems, m.cursor, m.offset, col2Width, listHeight)
	previewCol := m.renderColumn(m.previewItems, -1, 0, col3Width, listHeight)

	return header + "\n" + separator + "\n" + m.joinColumns(parentCol, currentCol, previewCol)
}

func (m Model) renderEmptyColumn(width int, height int) []string {
	lines := make([]string, height)
	for i := 0; i < height; i++ {
		lines[i] = strings.Repeat(" ", width)
	}
	return lines
}

func (m Model) renderColumn(
	items []Entry,
	cursor int,
	offset int,
	width int,
	height int,
) []string {
	lines := make([]string, height)

	for i := 0; i < height; i++ {
		idx := i + offset
		if idx < len(items) {
			entry := items[idx]
			name := entry.Name
			if entry.IsDir {
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

func (m Model) joinColumns(col1, col2, col3 []string) string {
	var sb strings.Builder

	maxLen := max(len(col1), len(col2), len(col3))
	for i := 0; i < maxLen; i++ {
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
