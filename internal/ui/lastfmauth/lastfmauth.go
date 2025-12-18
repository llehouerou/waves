// Package lastfmauth provides a Last.fm account linking popup.
package lastfmauth

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// Compile-time check that Model implements popup.Popup.
var _ popup.Popup = (*Model)(nil)

// authState represents the current authentication state.
type authState int

const (
	stateNotLinked authState = iota
	stateWaitingCallback
	stateLinked
	stateError
)

// ActionMsg is sent when an action occurs in the popup.
type ActionMsg struct {
	Action Action
}

// Action represents an action from the popup.
type Action int

const (
	// ActionNone indicates no action.
	ActionNone Action = iota
	// ActionClose indicates the popup should be closed.
	ActionClose
	// ActionStartAuth indicates authentication should start.
	ActionStartAuth
	// ActionUnlink indicates the account should be unlinked.
	ActionUnlink
	// ActionConfirmAuth indicates user manually confirmed authorization.
	ActionConfirmAuth
)

// Key constants.
const keyEsc = "esc"

func titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.T().Primary)
}

func labelStyle() lipgloss.Style {
	return styles.T().S().Base
}

func valueStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.T().Secondary)
}

func hintStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func errorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.T().Error)
}

func successStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.T().Success)
}

// Model is the Last.fm authentication popup.
type Model struct {
	ui.Base
	state    authState
	username string // When linked
	errMsg   string // When error
}

// New creates a new Last.fm auth popup.
func New() Model {
	return Model{
		state: stateNotLinked,
	}
}

// SetSession sets the current session state.
func (m *Model) SetSession(session *state.LastfmSession) {
	if session != nil {
		m.state = stateLinked
		m.username = session.Username
	} else {
		m.state = stateNotLinked
		m.username = ""
	}
	m.errMsg = ""
}

// SetWaitingCallback sets the popup to waiting state.
func (m *Model) SetWaitingCallback() {
	m.state = stateWaitingCallback
	m.errMsg = ""
}

// SetError sets an error message.
func (m *Model) SetError(err string) {
	m.state = stateError
	m.errMsg = err
}

// Init implements popup.Popup.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch m.state {
	case stateNotLinked, stateError:
		return m.handleNotLinkedKey(keyMsg)
	case stateWaitingCallback:
		return m.handleWaitingKey(keyMsg)
	case stateLinked:
		return m.handleLinkedKey(keyMsg)
	}

	return m, nil
}

func (m *Model) handleNotLinkedKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m, func() tea.Msg {
			return ActionMsg{Action: ActionStartAuth}
		}
	case keyEsc:
		return m, func() tea.Msg {
			return ActionMsg{Action: ActionClose}
		}
	}
	return m, nil
}

func (m *Model) handleWaitingKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// User manually confirms they authorized
		return m, func() tea.Msg {
			return ActionMsg{Action: ActionConfirmAuth}
		}
	case keyEsc:
		// Cancel waiting
		return m, func() tea.Msg {
			return ActionMsg{Action: ActionClose}
		}
	}
	return m, nil
}

func (m *Model) handleLinkedKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "u", "U":
		return m, func() tea.Msg {
			return ActionMsg{Action: ActionUnlink}
		}
	case keyEsc:
		return m, func() tea.Msg {
			return ActionMsg{Action: ActionClose}
		}
	}
	return m, nil
}

// View implements popup.Popup.
func (m *Model) View() string {
	title := titleStyle().Render("Last.fm Settings")

	var content string

	switch m.state {
	case stateNotLinked:
		content = m.viewNotLinked()
	case stateWaitingCallback:
		content = m.viewWaiting()
	case stateLinked:
		content = m.viewLinked()
	case stateError:
		content = m.viewError()
	}

	return title + "\n\n" + content
}

func (m *Model) viewNotLinked() string {
	status := labelStyle().Render("Status: ") + valueStyle().Render("Not linked")
	hint := hintStyle().Render("Press Enter to link your Last.fm account")
	footer := hintStyle().Render("[Enter] Link  [Esc] Close")

	return status + "\n\n" + hint + "\n\n" + footer
}

func (m *Model) viewWaiting() string {
	status := labelStyle().Render("Status: ") + valueStyle().Render("Authorizing...")
	info := labelStyle().Render("A browser window has opened.\nAuthorize Waves on Last.fm, then press Enter.")
	footer := hintStyle().Render("[Enter] I've authorized  [Esc] Cancel")

	return status + "\n\n" + info + "\n\n" + footer
}

func (m *Model) viewLinked() string {
	status := labelStyle().Render("Status: ") + successStyle().Render("Linked")
	username := labelStyle().Render("Username: ") + valueStyle().Render(m.username)
	scrobbling := labelStyle().Render("Scrobbling: ") + successStyle().Render("Active")
	footer := hintStyle().Render("[u] Unlink  [Esc] Close")

	return status + "\n" + username + "\n" + scrobbling + "\n\n" + footer
}

func (m *Model) viewError() string {
	status := labelStyle().Render("Status: ") + errorStyle().Render("Error")
	errMsg := errorStyle().Render(m.errMsg)
	hint := hintStyle().Render("Press Enter to try again")
	footer := hintStyle().Render("[Enter] Retry  [Esc] Close")

	return fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", status, errMsg, hint, footer)
}

// Client returns a new Last.fm client command initiator.
type Client interface {
	GetToken() (string, error)
	GetAuthURL(token string) string
	GetSession(token string) (username, sessionKey string, err error)
}

// StartAuthCmd starts the authentication flow.
func StartAuthCmd(client *lastfm.Client) tea.Cmd {
	return lastfm.GetTokenCmd(client)
}
