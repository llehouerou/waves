package popup

import tea "github.com/charmbracelet/bubbletea"

// Popup defines the contract for modal popup components.
type Popup interface {
	// Init returns any initial command (e.g., focus text input).
	Init() tea.Cmd

	// Update handles messages and returns updated popup + command.
	Update(msg tea.Msg) (Popup, tea.Cmd)

	// View renders the popup content (without outer border/centering).
	View() string

	// SetSize sets the available dimensions for the popup content.
	SetSize(width, height int)
}
