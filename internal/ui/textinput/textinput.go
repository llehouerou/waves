// Package textinput provides a simple text input popup component.
package textinput

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// Compile-time check that Model implements popup.Popup.
var _ popup.Popup = (*Model)(nil)

func inputStyle() lipgloss.Style {
	return styles.T().S().Base
}

func titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.T().Primary)
}

func hintStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

// Model is a simple text input popup.
type Model struct {
	ui.Base
	title   string
	text    string
	context any // passed through to Result action
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
	m.SetSize(width, height)
}

// Reset clears the input state.
func (m *Model) Reset() {
	m.title = ""
	m.text = ""
	m.context = nil
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
			ctx := m.context
			return m, func() tea.Msg {
				return ActionMsg(Result{Canceled: true, Context: ctx})
			}

		case "enter":
			text := m.text
			ctx := m.context
			return m, func() tea.Msg {
				return ActionMsg(Result{Text: text, Context: ctx})
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
	if m.Width() == 0 || m.Height() == 0 {
		return ""
	}

	// Title
	title := titleStyle().Render(m.title)

	// Input field
	cursor := "█"
	input := inputStyle().Render("> "+m.text) + cursor

	// Hint
	hint := hintStyle().Render("Enter: confirm, Esc: cancel")

	// Build content
	content := title + "\n\n" + input + "\n\n" + hint

	return content
}
