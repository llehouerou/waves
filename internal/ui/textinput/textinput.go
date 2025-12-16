// Package textinput provides a simple text input popup component.
package textinput

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/popup"
)

// Compile-time check that Model implements popup.Popup.
var _ popup.Popup = (*Model)(nil)

var (
	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// ResultMsg is emitted when text input completes or is canceled.
type ResultMsg struct {
	Text     string
	Context  any  // User-provided context passed through
	Canceled bool // True if user pressed Escape
}

// Model is a simple text input popup.
type Model struct {
	title   string
	text    string
	context any // passed through to ResultMsg
	width   int
	height  int
}

// New creates a new text input model.
func New() Model {
	return Model{}
}

// Start initializes the input with a title and optional initial text.
func (m *Model) Start(title, initialText string, context any, width, height int) {
	m.title = title
	m.text = initialText
	m.context = context
	m.width = width
	m.height = height
}

// Reset clears the input state.
func (m *Model) Reset() {
	m.title = ""
	m.text = ""
	m.context = nil
}

// SetSize implements popup.Popup.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init implements popup.Popup.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return ResultMsg{Canceled: true, Context: m.context}
			}

		case "enter":
			return m, func() tea.Msg {
				return ResultMsg{Text: m.text, Context: m.context}
			}

		case "backspace":
			if m.text != "" {
				m.text = m.text[:len(m.text)-1]
			}

		default:
			// Only add printable characters
			if len(msg.String()) == 1 && msg.String()[0] >= 32 {
				m.text += msg.String()
			}
		}
	}

	return m, nil
}

// View implements popup.Popup.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	popupW := min(50, m.width-4)
	innerW := popupW - 2

	// Title
	title := titleStyle.Render(m.title)

	// Input field
	cursor := "█"
	input := inputStyle.Render("> "+m.text) + cursor

	// Hint
	hint := hintStyle.Render("Enter: confirm, Esc: cancel")

	// Build content
	separator := strings.Repeat("─", innerW)
	content := title + "\n" + separator + "\n" + input + "\n" + separator + "\n" + hint

	// Style popup
	box := popupStyle.Width(innerW).Render(content)

	return popup.Center(box, m.width, m.height)
}
