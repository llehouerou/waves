// Package confirm provides a yes/no confirmation popup component.
package confirm

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/popup"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))
)

// ResultMsg is emitted when the user confirms or cancels.
type ResultMsg struct {
	Confirmed      bool
	Context        any // User-provided context passed through
	SelectedOption int // Index of selected option (for multi-option mode)
}

// Model is a yes/no confirmation popup.
type Model struct {
	title          string
	message        string
	context        any
	width          int
	height         int
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
	m.width = width
	m.height = height
	m.active = true
	m.options = nil
	m.selectedOption = 0
}

// ShowWithOptions displays the confirmation popup with multiple options.
func (m *Model) ShowWithOptions(title, message string, options []string, context any, width, height int) {
	m.title = title
	m.message = message
	m.context = context
	m.width = width
	m.height = height
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

// Update handles key events.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Multi-option mode
		if len(m.options) > 0 {
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
					return ResultMsg{Confirmed: confirmed, Context: ctx, SelectedOption: selected}
				}
			case "esc":
				m.active = false
				ctx := m.context
				return m, func() tea.Msg {
					return ResultMsg{Confirmed: false, Context: ctx, SelectedOption: len(m.options) - 1}
				}
			}
			return m, nil
		}

		// Yes/no mode
		switch msg.String() {
		case "enter", "y", "Y":
			m.active = false
			ctx := m.context
			return m, func() tea.Msg {
				return ResultMsg{Confirmed: true, Context: ctx}
			}

		case "esc", "n", "N":
			m.active = false
			ctx := m.context
			return m, func() tea.Msg {
				return ResultMsg{Confirmed: false, Context: ctx}
			}
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

// View renders the confirmation popup.
func (m Model) View() string {
	if !m.active || m.width == 0 || m.height == 0 {
		return ""
	}

	popupW := min(50, m.width-4)
	innerW := popupW - 2

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

	// Style popup
	box := popupStyle.Width(innerW).Render(content)

	return popup.Center(box, m.width, m.height)
}
