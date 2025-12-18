// Package confirm provides a yes/no confirmation popup component.
package confirm

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// Compile-time check that Model implements popup.Popup.
var _ popup.Popup = (*Model)(nil)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// Model is a yes/no confirmation popup.
type Model struct {
	ui.Base
	title          string
	message        string
	context        any
	active         bool
	options        []string // Multi-option mode
	selectedOption int      // Currently selected option index
}

// New creates a new confirmation model.
func New() Model {
	return Model{}
}

// Show displays the confirmation popup (yes/no mode).
func (m *Model) Show(title, message string, context any, width, height int) {
	m.title = title
	m.message = message
	m.context = context
	m.SetSize(width, height)
	m.active = true
	m.options = nil
	m.selectedOption = 0
}

// ShowWithOptions displays the confirmation popup with multiple options.
func (m *Model) ShowWithOptions(title, message string, options []string, context any, width, height int) {
	m.title = title
	m.message = message
	m.context = context
	m.SetSize(width, height)
	m.active = true
	m.options = options
	m.selectedOption = 0
}

// Reset clears the confirmation state.
func (m *Model) Reset() {
	m.title = ""
	m.message = ""
	m.context = nil
	m.active = false
	m.options = nil
	m.selectedOption = 0
}

// Active returns whether the confirmation is currently shown.
func (m Model) Active() bool {
	return m.active
}

// Init implements popup.Popup.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	// Multi-option mode
	if len(m.options) > 0 {
		return m.handleMultiOptionKey(keyMsg)
	}

	// Yes/no mode
	return m.handleYesNoKey(keyMsg)
}

func (m *Model) handleMultiOptionKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedOption > 0 {
			m.selectedOption--
		}
	case "down", "j":
		if m.selectedOption < len(m.options)-1 {
			m.selectedOption++
		}
	case "enter":
		m.active = false
		ctx := m.context
		selected := m.selectedOption
		// Last option is always "Cancel"
		confirmed := selected < len(m.options)-1
		return m, func() tea.Msg {
			return ActionMsg(Result{Confirmed: confirmed, Context: ctx, SelectedOption: selected})
		}
	case "esc":
		m.active = false
		ctx := m.context
		numOptions := len(m.options)
		return m, func() tea.Msg {
			return ActionMsg(Result{Confirmed: false, Context: ctx, SelectedOption: numOptions - 1})
		}
	}
	return m, nil
}

func (m *Model) handleYesNoKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		m.active = false
		ctx := m.context
		return m, func() tea.Msg {
			return ActionMsg(Result{Confirmed: true, Context: ctx})
		}

	case "esc", "n", "N":
		m.active = false
		ctx := m.context
		return m, func() tea.Msg {
			return ActionMsg(Result{Confirmed: false, Context: ctx})
		}
	}
	return m, nil
}

var (
	optionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	selectedOptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true)
)

// View implements popup.Popup.
func (m *Model) View() string {
	if !m.active || m.Width() == 0 || m.Height() == 0 {
		return ""
	}

	// Title
	title := titleStyle.Render(m.title)

	// Message
	message := messageStyle.Render(m.message)

	var content string

	// Multi-option mode
	if len(m.options) > 0 {
		var optionLines []string
		for i, opt := range m.options {
			prefix := "  "
			style := optionStyle
			if i == m.selectedOption {
				prefix = "> "
				style = selectedOptionStyle
			}
			optionLines = append(optionLines, style.Render(prefix+opt))
		}
		optionsView := lipgloss.JoinVertical(lipgloss.Left, optionLines...)

		hint := hintStyle.Render("↑↓/jk navigate · enter select")
		content = title + "\n\n" + message + "\n\n" + optionsView + "\n\n" + hint
	} else {
		// Yes/no mode
		hint := hintStyle.Render("Enter/Y: confirm, Esc/N: cancel")
		content = title + "\n\n" + message + "\n\n" + hint
	}

	return content
}
