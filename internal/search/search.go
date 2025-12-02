package search

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/llehouerou/waves/internal/ui/popup"
)

const maxVisibleResults = 20

var (
	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// ResultMsg is emitted when the search completes (selection or cancel).
// Root model should navigate to the selected item or reset search state.
// Emitted on Enter (selection) or Escape (cancel).
type ResultMsg struct {
	Item     Item // The selected item (nil if canceled)
	Canceled bool // True if user pressed Escape
}

// Model is a generic fuzzy search popup.
type Model struct {
	items   []Item
	matches []fuzzy.Match
	query   string
	cursor  int
	offset  int
	loading bool
	width   int
	height  int
}

// New creates a new search model.
func New() Model {
	return Model{}
}

// SetItems updates the items to search.
func (m *Model) SetItems(items []Item) {
	m.items = items
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
	m.matches = nil
	m.loading = false
}

func (m *Model) updateMatches() {
	if m.query == "" {
		// Show all items when query is empty
		m.matches = make([]fuzzy.Match, len(m.items))
		for i := range m.items {
			m.matches[i] = fuzzy.Match{Index: i}
		}
	} else {
		m.matches = fuzzy.FindFrom(m.query, items(m.items))
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

func (m Model) visibleHeight() int {
	// Account for border (2) + input line (1) + separator (1)
	h := max(m.popupHeight()-4, 1)
	return min(h, maxVisibleResults)
}

func (m Model) popupWidth() int {
	w := m.width * 60 / 100
	if w < 40 {
		w = min(40, m.width-4)
	}
	return w
}

func (m Model) popupHeight() int {
	h := m.height * 50 / 100
	if h < 10 {
		h = min(10, m.height-2)
	}
	return h
}

func (m Model) emptyMessage() string {
	switch {
	case m.loading:
		return "Scanning..."
	case m.query != "":
		return "No matches"
	default:
		return "Type to search..."
	}
}

func (m Model) formatResultLine(item Item, innerW int, isCursor bool) string {
	prefix := "  "
	if isCursor {
		prefix = "> "
	}

	// Check if item supports two-column display
	twoCol, ok := item.(TwoColumnItem)
	if !ok {
		// Fallback to single column display
		text := item.DisplayText()
		if lipgloss.Width(text) > innerW-4 {
			text = text[:innerW-7] + "..."
		}
		return prefix + text
	}

	left := twoCol.LeftColumn()
	right := twoCol.RightColumn()
	availW := innerW - 4 // account for prefix and padding

	if right == "" {
		// No right column, just show left
		if lipgloss.Width(left) > availW {
			left = left[:availW-3] + "..."
		}
		return prefix + left
	}

	// Truncate left column if needed, leaving space for right
	rightW := lipgloss.Width(right)
	maxLeftW := availW - rightW - 2 // 2 for gap
	if lipgloss.Width(left) > maxLeftW {
		if maxLeftW > 3 {
			left = left[:maxLeftW-3] + "..."
		} else if maxLeftW > 0 {
			left = left[:maxLeftW]
		}
	}

	// Build line with left-aligned name and right-aligned path
	gap := max(1, availW-lipgloss.Width(left)-rightW)
	return prefix + left + strings.Repeat(" ", gap) + dimStyle.Render(right)
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

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	popupW := m.popupWidth()
	innerW := popupW - 2 // account for border

	// Input line
	prompt := "> "
	input := inputStyle.Render(prompt + m.query)

	// Separator
	separator := strings.Repeat("─", innerW)

	// Results
	visible := m.visibleHeight()
	var resultLines []string

	if len(m.matches) == 0 {
		resultLines = append(resultLines, dimStyle.Render(m.emptyMessage()))
	} else {
		end := min(m.offset+visible, len(m.matches))
		for i := m.offset; i < end; i++ {
			match := m.matches[i]
			item := m.items[match.Index]
			isCursor := i == m.cursor
			line := m.formatResultLine(item, innerW, isCursor)

			if isCursor {
				resultLines = append(resultLines, selectedStyle.Render(line))
			} else {
				resultLines = append(resultLines, normalStyle.Render(line))
			}
		}
	}

	// Loading indicator in input line
	inputLine := input
	if m.loading {
		spinnerChar := "◐" // simple spinner
		inputLine = input + dimStyle.Render(" "+spinnerChar)
	}

	// Pad result lines to fill popup height
	for len(resultLines) < visible {
		resultLines = append(resultLines, "")
	}

	// Build popup content
	content := inputLine + "\n" + separator + "\n" + strings.Join(resultLines, "\n")

	// Style the popup with border
	box := popupStyle.Width(innerW).Render(content)

	// Center in terminal
	return popup.Center(box, m.width, m.height)
}
