package navigator

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/cursor"
)

type Model[T Node] struct {
	source       Source[T]
	current      T
	currentItems []T
	previewItems []T
	previewLines []string // Custom preview lines (from PreviewProvider)
	parentItems  []T      // Items in the parent directory (for left column)
	parentCursor int      // Index of current in parent's children
	cursor       cursor.Cursor
	width        int
	height       int
	focused      bool
	favorites    map[int64]bool // Track IDs that are favorited
}

func New[T Node](source Source[T]) (Model[T], error) {
	m := Model[T]{
		source:  source,
		current: source.Root(),
		cursor:  cursor.New(ui.ScrollMargin),
	}

	if err := m.refresh(); err != nil {
		return Model[T]{}, err
	}

	return m, nil
}

// SetFocused sets whether the navigator is focused.
func (m *Model[T]) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether the navigator is focused.
func (m Model[T]) IsFocused() bool {
	return m.focused
}

// SetFavorites sets the map of favorite track IDs for display.
func (m *Model[T]) SetFavorites(favorites map[int64]bool) {
	m.favorites = favorites
}

// IsFavorite returns true if the given track ID is in the favorites map.
func (m Model[T]) IsFavorite(trackID int64) bool {
	if m.favorites == nil {
		return false
	}
	return m.favorites[trackID]
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
		m.cursor.Reset()
		_ = m.refresh()
	} else {
		// Navigate to parent directory and select the file
		parent := m.source.Parent(node)
		if parent == nil {
			return false
		}
		m.current = *parent
		m.cursor.Reset()
		_ = m.refresh()
		m.FocusByName(node.DisplayName())
	}

	return true
}

// FocusByID navigates to the parent of the node and focuses on it.
func (m *Model[T]) FocusByID(id string) bool {
	node, ok := m.source.NodeFromID(id)
	if !ok {
		return false
	}

	parent := m.source.Parent(node)
	if parent == nil {
		return false
	}

	m.current = *parent
	m.cursor.Reset()
	_ = m.refresh()
	m.focusNode(id)

	return true
}

// CurrentPath returns the display path of the current folder.
func (m Model[T]) CurrentPath() string {
	return m.source.DisplayPath(m.current)
}

// Current returns the current container node.
func (m Model[T]) Current() T {
	return m.current
}

// CurrentItems returns the items in the current directory.
func (m Model[T]) CurrentItems() []T {
	return m.currentItems
}

func (m Model[T]) navigationChangedCmd() tea.Cmd {
	currentPath := m.CurrentPath()
	selectedName := m.SelectedName()
	return func() tea.Msg {
		return ActionMsg(NavigationChanged{
			CurrentPath:  currentPath,
			SelectedName: selectedName,
		})
	}
}

func (m Model[T]) Init() tea.Cmd {
	return nil
}

func (m Model[T]) Update(msg tea.Msg) (Model[T], tea.Cmd) {
	var navChanged bool

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		firstSize := m.height == 0
		m.width = msg.Width
		m.height = msg.Height
		if firstSize {
			// Center cursor on first size (startup with restored selection)
			m.cursor.Center(len(m.currentItems), m.listHeight())
		}

	case tea.MouseMsg:
		navChanged = m.handleMouse(msg)

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor.Pos() > 0 {
				m.cursor.Move(-1, len(m.currentItems), m.listHeight())
				m.updatePreview()
				navChanged = true
			}

		case "down", "j":
			if m.cursor.Pos() < len(m.currentItems)-1 {
				m.cursor.Move(1, len(m.currentItems), m.listHeight())
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
				selected := m.currentItems[m.cursor.Pos()]
				if selected.IsContainer() {
					m.current = selected
					m.cursor.Reset()
					_ = m.refresh()
					navChanged = true
				}
			}

		case "g", "home":
			if m.cursor.Pos() > 0 {
				m.cursor.JumpStart()
				m.updatePreview()
				navChanged = true
			}

		case "G", "end":
			if m.cursor.Pos() < len(m.currentItems)-1 {
				m.cursor.JumpEnd(len(m.currentItems), m.listHeight())
				m.updatePreview()
				navChanged = true
			}

		case "ctrl+d":
			if m.cursor.Pos() < len(m.currentItems)-1 {
				m.cursor.Move(m.listHeight()/2, len(m.currentItems), m.listHeight())
				m.updatePreview()
				navChanged = true
			}

		case "ctrl+u":
			if m.cursor.Pos() > 0 {
				m.cursor.Move(-m.listHeight()/2, len(m.currentItems), m.listHeight())
				m.updatePreview()
				navChanged = true
			}
		}
	}

	if navChanged {
		return m, m.navigationChangedCmd()
	}
	return m, nil
}

func (m *Model[T]) handleMouse(msg tea.MouseMsg) bool {
	// Handle wheel scroll
	if msg.Button == tea.MouseButtonWheelUp {
		if m.cursor.Pos() > 0 {
			m.cursor.Move(-1, len(m.currentItems), m.listHeight())
			m.updatePreview()
			return true
		}
		return false
	}

	if msg.Button == tea.MouseButtonWheelDown {
		if m.cursor.Pos() < len(m.currentItems)-1 {
			m.cursor.Move(1, len(m.currentItems), m.listHeight())
			m.updatePreview()
			return true
		}
		return false
	}

	// Handle clicks (only on press)
	if msg.Action != tea.MouseActionPress {
		return false
	}

	if msg.Button == tea.MouseButtonMiddle {
		// Middle click: navigate into container (tracks handled by app)
		if len(m.currentItems) == 0 {
			return false
		}
		selected := m.currentItems[m.cursor.Pos()]
		if !selected.IsContainer() {
			return false
		}
		m.current = selected
		m.cursor.Reset()
		_ = m.refresh()
		return true
	}

	if msg.Button == tea.MouseButtonRight {
		// Right click: navigate to parent
		parent := m.source.Parent(m.current)
		if parent == nil {
			return false
		}
		prevID := m.current.ID()
		m.current = *parent
		_ = m.refresh()
		m.focusNode(prevID)
		return true
	}

	return false
}

// listHeight returns the available height for the list.
func (m Model[T]) listHeight() int {
	return m.height - ui.PanelOverhead
}
