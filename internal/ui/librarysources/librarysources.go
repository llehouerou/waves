// Package librarysources provides a popup for managing library source paths.
package librarysources

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/cursor"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// Compile-time check that Model implements popup.Popup.
var _ popup.Popup = (*Model)(nil)

type mode int

const (
	modeList mode = iota
	modeAdd
	modeConfirm
)

const keyEsc = "esc"

// SourceAddedMsg is emitted when a source is added.
type SourceAddedMsg struct {
	Path string
}

// SourceRemovedMsg is emitted when a source is removed.
type SourceRemovedMsg struct {
	Path string
}

// RequestTrackCountMsg is emitted when the popup needs the track count for a path.
type RequestTrackCountMsg struct {
	Path string
}

// CloseMsg is emitted when the popup should close.
type CloseMsg struct{}

var (
	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("252"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// Model is the library sources popup.
type Model struct {
	sources      []string
	cursor       cursor.Cursor
	mode         mode
	inputText    string
	trackCount   int // for confirm mode
	width        int
	height       int
	errorMessage string
}

// New creates a new library sources model.
func New() Model {
	return Model{
		cursor: cursor.New(0),
	}
}

// SetSources sets the list of sources to display.
func (m *Model) SetSources(sources []string) {
	m.sources = sources
	m.cursor.ClampToBounds(len(sources))
}

// SetSize sets the terminal dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetTrackCount sets the track count for confirmation.
func (m *Model) SetTrackCount(count int) {
	m.trackCount = count
}

// Reset clears the model state.
func (m *Model) Reset() {
	m.mode = modeList
	m.inputText = ""
	m.errorMessage = ""
}

// SelectedPath returns the currently selected path.
func (m Model) SelectedPath() string {
	pos := m.cursor.Pos()
	if pos >= 0 && pos < len(m.sources) {
		return m.sources[pos]
	}
	return ""
}

// EnterConfirmMode switches to confirmation mode with the given track count.
func (m *Model) EnterConfirmMode(trackCount int) {
	m.trackCount = trackCount
	m.mode = modeConfirm
}

// Init implements popup.Popup.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		m.errorMessage = ""

		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeAdd:
			return m.updateAdd(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		}
	}

	return m, nil
}

func (m *Model) updateList(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		return m, func() tea.Msg { return CloseMsg{} }

	case "j", "down":
		m.cursor.Move(1, len(m.sources), 0)

	case "k", "up":
		m.cursor.Move(-1, len(m.sources), 0)

	case "a":
		m.mode = modeAdd
		m.inputText = ""

	case "d":
		if len(m.sources) > 0 {
			path := m.SelectedPath()
			return m, func() tea.Msg { return RequestTrackCountMsg{Path: path} }
		}
	}

	return m, nil
}

func (m *Model) updateAdd(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		m.mode = modeList
		m.inputText = ""

	case "enter":
		path := strings.TrimSpace(m.inputText)
		if path == "" {
			m.mode = modeList
			return m, nil
		}

		// Expand ~ to home directory
		if strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				path = home + path[1:]
			}
		}

		// Validate path exists
		info, err := os.Stat(path)
		if err != nil {
			m.errorMessage = "Path does not exist"
			return m, nil
		}
		if !info.IsDir() {
			m.errorMessage = "Path is not a directory"
			return m, nil
		}

		m.mode = modeList
		m.inputText = ""
		return m, func() tea.Msg { return SourceAddedMsg{Path: path} }

	case "backspace":
		if m.inputText != "" {
			m.inputText = m.inputText[:len(m.inputText)-1]
		}

	default:
		if len(msg.String()) == 1 && msg.String()[0] >= 32 {
			m.inputText += msg.String()
		}
	}

	return m, nil
}

func (m *Model) updateConfirm(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		path := m.SelectedPath()
		m.mode = modeList
		return m, func() tea.Msg { return SourceRemovedMsg{Path: path} }

	case "n", "N", keyEsc:
		m.mode = modeList
	}

	return m, nil
}

// View implements popup.Popup.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	p := popup.New()
	p.Title = "Library Sources"
	p.Width = min(60, m.width-4)

	switch m.mode {
	case modeList:
		p.Content = m.renderList()
		p.Footer = "a: add  d: delete  Esc: close"

	case modeAdd:
		p.Content = m.renderAdd()
		p.Footer = "Enter: confirm  Esc: cancel"

	case modeConfirm:
		p.Content = m.renderConfirm()
		p.Footer = "y: confirm  n: cancel"
	}

	return p.Render(m.width, m.height)
}

func (m Model) renderList() string {
	if len(m.sources) == 0 {
		return hintStyle.Render("No sources configured.\nPress 'a' to add a path.")
	}

	lines := make([]string, len(m.sources))
	cursorPos := m.cursor.Pos()
	for i, source := range m.sources {
		style := normalStyle
		prefix := "  "
		if i == cursorPos {
			style = selectedStyle
			prefix = "> "
		}
		lines[i] = style.Render(prefix + source)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderAdd() string {
	caret := "█"
	input := inputStyle.Render("> "+m.inputText) + caret

	result := "Enter path (supports ~):\n" + input

	if m.errorMessage != "" {
		result += "\n" + warningStyle.Render(m.errorMessage)
	}

	return result
}

func (m Model) renderConfirm() string {
	path := m.SelectedPath()
	msg := fmt.Sprintf("Remove source:\n%s\n\n", path)

	if m.trackCount > 0 {
		msg += warningStyle.Render(fmt.Sprintf("This will remove %d tracks from the library.", m.trackCount))
	} else {
		msg += "No tracks will be affected."
	}

	return msg
}
