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
		source: source,
		cursor: cursor.New(ui.ScrollMargin),
	}

	if err := m.moveTo(source.Root()); err != nil {
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
// It returns false for an ID that names no node, and the error of a lookup
// that failed or of a listing of the container to go to that failed: the
// navigator does not move then.
func (m *Model[T]) NavigateTo(id string) (bool, error) {
	node, ok, err := m.source.NodeFromID(id)
	if err != nil || !ok {
		return false, err
	}

	if node.IsContainer() {
		// Navigate into the directory
		if err := m.moveTo(node); err != nil {
			return false, err
		}
		return true, nil
	}

	// Navigate to parent directory and select the file
	parent := m.source.Parent(node)
	if parent == nil {
		return false, nil
	}
	if err := m.moveTo(*parent); err != nil {
		return false, err
	}
	m.FocusByName(node.DisplayName())
	return true, nil
}

// FocusByID navigates to the parent of the node and focuses on it. It
// returns false for an ID that names no node, and the error of a lookup that
// failed or of a listing of the parent that failed: the navigator does not
// move then.
func (m *Model[T]) FocusByID(id string) (bool, error) {
	node, ok, err := m.source.NodeFromID(id)
	if err != nil || !ok {
		return false, err
	}

	parent := m.source.Parent(node)
	if parent == nil {
		return false, nil
	}

	if err := m.moveTo(*parent); err != nil {
		return false, err
	}
	m.focusNode(id)

	return true, nil
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
	var err error

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
		navChanged, err = m.handleMouse(msg)

	case tea.KeyMsg:
		navChanged, err = m.handleKey(msg.String())
	}

	if err != nil {
		return m, func() tea.Msg { return ActionMsg(NavigationFailed{Err: err}) }
	}
	if navChanged {
		return m, m.navigationChangedCmd()
	}
	return m, nil
}

// handleMouse handles mouse input for navigation. It returns true if the
// navigation state changed, and the error of a move it refused.
func (m *Model[T]) handleMouse(msg tea.MouseMsg) (bool, error) {
	// Handle wheel scroll
	if msg.Button == tea.MouseButtonWheelUp {
		if m.cursor.Pos() > 0 {
			m.cursor.Move(-1, len(m.currentItems), m.listHeight())
			m.updatePreview()
			return true, nil
		}
		return false, nil
	}

	if msg.Button == tea.MouseButtonWheelDown {
		if m.cursor.Pos() < len(m.currentItems)-1 {
			m.cursor.Move(1, len(m.currentItems), m.listHeight())
			m.updatePreview()
			return true, nil
		}
		return false, nil
	}

	// Handle clicks (only on press)
	if msg.Action != tea.MouseActionPress {
		return false, nil
	}

	if msg.Button == tea.MouseButtonMiddle {
		// Middle click: navigate into container (tracks handled by app)
		return m.enterSelected()
	}

	if msg.Button == tea.MouseButtonRight {
		// Right click: navigate to parent
		return m.goToParent()
	}

	return false, nil
}

// handleKey handles keyboard input for navigation. It returns true if the
// navigation state changed, and the error of a move it refused.
func (m *Model[T]) handleKey(key string) (bool, error) {
	// Handle common list navigation keys via cursor
	oldPos := m.cursor.Pos()
	if m.cursor.HandleKey(key, len(m.currentItems), m.listHeight()) {
		if m.cursor.Pos() != oldPos {
			m.updatePreview()
			return true, nil
		}
		return false, nil
	}

	// Handle container navigation
	switch key {
	case "left", "h":
		return m.goToParent()
	case "right", "l", "enter":
		return m.enterSelected()
	}

	return false, nil
}

// enterSelected moves into the container under the cursor. It returns true
// if it moved, and the error of a listing that failed: it stays then.
func (m *Model[T]) enterSelected() (bool, error) {
	if len(m.currentItems) == 0 {
		return false, nil
	}
	selected := m.currentItems[m.cursor.Pos()]
	if !selected.IsContainer() {
		return false, nil
	}
	if err := m.moveTo(selected); err != nil {
		return false, err
	}
	return true, nil
}

// goToParent moves to the parent of the current container, its cursor on the
// container it comes from. It returns true if it moved, and the error of a
// listing that failed: it stays then.
func (m *Model[T]) goToParent() (bool, error) {
	parent := m.source.Parent(m.current)
	if parent == nil {
		return false, nil
	}
	prevID := m.current.ID()
	if err := m.moveTo(*parent); err != nil {
		return false, err
	}
	m.focusNode(prevID)
	return true, nil
}

// listHeight returns the available height for the list, 0 when the panel has
// no room for one.
func (m Model[T]) listHeight() int {
	return max(m.height-ui.PanelOverhead, 0)
}
