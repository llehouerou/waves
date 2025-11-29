package search

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
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

// ResultMsg is sent when user selects an item.
type ResultMsg struct {
	Item     Item
	Canceled bool
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
		var msg string
		switch {
		case m.loading:
			msg = "Scanning..."
		case m.query != "":
			msg = "No matches"
		default:
			msg = "Type to search..."
		}
		resultLines = append(resultLines, dimStyle.Render(msg))
	} else {
		end := min(m.offset+visible, len(m.matches))
		for i := m.offset; i < end; i++ {
			match := m.matches[i]
			item := m.items[match.Index]
			text := item.DisplayText()

			// Truncate if needed
			if lipgloss.Width(text) > innerW-4 {
				text = text[:innerW-7] + "..."
			}

			if i == m.cursor {
				resultLines = append(resultLines, selectedStyle.Render("> "+text))
			} else {
				resultLines = append(resultLines, normalStyle.Render("  "+text))
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

	// Style and center the popup
	popup := popupStyle.Width(innerW).Render(content)

	// Center in terminal
	popupLines := strings.Split(popup, "\n")
	actualH := len(popupLines)

	padTop := (m.height - actualH) / 2
	padLeft := (m.width - popupW) / 2

	if padTop < 0 {
		padTop = 0
	}
	if padLeft < 0 {
		padLeft = 0
	}

	var result strings.Builder
	for range padTop {
		result.WriteString(strings.Repeat(" ", m.width) + "\n")
	}
	for _, line := range popupLines {
		result.WriteString(strings.Repeat(" ", padLeft))
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}
