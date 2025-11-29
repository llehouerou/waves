package navigator

import (
	tea "github.com/charmbracelet/bubbletea"
)

// NavigationChangedMsg is sent when the current folder or selection changes.
type NavigationChangedMsg struct {
	CurrentPath  string
	SelectedName string
}

type Model[T Node] struct {
	source       Source[T]
	current      T
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

	m.offset = max(m.cursor-listHeight/2, 0)
	maxOffset := max(len(m.currentItems)-listHeight, 0)
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

// CurrentItems returns the items in the current directory.
func (m Model[T]) CurrentItems() []T {
	return m.currentItems
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
