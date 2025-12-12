package search

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ResultMsg is emitted when the search completes (selection or cancel).
// Root model should navigate to the selected item or reset search state.
// Emitted on Enter (selection) or Escape (cancel).
type ResultMsg struct {
	Item     Item // The selected item (nil if canceled)
	Canceled bool // True if user pressed Escape
}

// Func is a function that searches for items matching a query.
// Used for FTS-backed search where filtering happens externally.
type Func func(query string) ([]Item, error)

// Model is a generic trigram search popup.
type Model struct {
	items      []Item
	matcher    *TrigramMatcher
	searchFunc Func // external search function (for FTS)
	matches    []Match
	query      string
	cursor     int
	offset     int
	loading    bool
	width      int
	height     int
}

// New creates a new search model.
func New() Model {
	return Model{}
}

// SetItems updates the items to search.
func (m *Model) SetItems(items []Item) {
	m.items = items
	m.matcher = NewTrigramMatcher(items)
	m.searchFunc = nil
	m.updateMatches()
}

// SetItemsWithMatcher sets items with a pre-built trigram matcher.
// Use this when the matcher is cached for faster search popup loading.
func (m *Model) SetItemsWithMatcher(items []Item, matcher *TrigramMatcher) {
	m.items = items
	m.matcher = matcher
	m.searchFunc = nil
	m.updateMatches()
}

// SetSearchFunc sets a search function for external filtering (e.g., FTS).
// The search function is called on each query change to get filtered items.
func (m *Model) SetSearchFunc(fn Func) {
	m.searchFunc = fn
	m.matcher = nil
	m.updateMatches()
}

// SetLoading sets the loading indicator.
func (m *Model) SetLoading(loading bool) {
	m.loading = loading
}

// Reset clears the search state.
func (m *Model) Reset() {
	m.query = ""
	m.cursor = 0
	m.offset = 0
	m.items = nil
	m.matcher = nil
	m.searchFunc = nil
	m.matches = nil
	m.loading = false
}

func (m *Model) updateMatches() {
	switch {
	case m.searchFunc != nil:
		// FTS-backed search: call external search function
		items, err := m.searchFunc(m.query)
		if err != nil {
			m.items = nil
			m.matches = nil
			return
		}
		m.items = items
		// Create 1:1 match mapping (items are already filtered/ranked by FTS)
		m.matches = make([]Match, len(items))
		for i := range items {
			m.matches[i] = Match{Index: i}
		}
	case m.matcher != nil:
		// Trigram-backed search: use in-memory matcher
		m.matches = m.matcher.Search(m.query)
	default:
		m.matches = nil
		return
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.matches) {
		m.cursor = max(0, len(m.matches)-1)
	}
	m.adjustOffset()
}

func (m *Model) adjustOffset() {
	visible := m.visibleHeight()
	if visible <= 0 {
		return
	}

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return ResultMsg{Canceled: true}
			}

		case "enter":
			var selected Item
			if len(m.matches) > 0 && m.cursor < len(m.matches) {
				selected = m.items[m.matches[m.cursor].Index]
			}
			return m, func() tea.Msg {
				return ResultMsg{Item: selected}
			}

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				m.adjustOffset()
			}

		case "down", "ctrl+n":
			if m.cursor < len(m.matches)-1 {
				m.cursor++
				m.adjustOffset()
			}

		case "backspace":
			if m.query != "" {
				m.query = m.query[:len(m.query)-1]
				m.cursor = 0
				m.offset = 0
				m.updateMatches()
			}

		default:
			// Only add printable characters
			if len(msg.String()) == 1 && msg.String()[0] >= 32 {
				m.query += msg.String()
				m.cursor = 0
				m.offset = 0
				m.updateMatches()
			}
		}
	}

	return m, nil
}
