package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/ui/lastfmauth"
	"github.com/llehouerou/waves/internal/ui/request"
)

// LastfmLinkStartedMsg carries the link the core began: the token to keep and
// the URL the user authorizes it at.
type LastfmLinkStartedMsg struct {
	Request request.ID
	Link    core.LastfmLink
	Err     error
}

// LastfmLinkedMsg carries the account the core linked.
type LastfmLinkedMsg struct {
	Request  request.ID
	Username string
	Err      error
}

// beginLastfmLink asks the core to begin linking an account.
func beginLastfmLink(lfm *core.Lastfm) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			link, err := lfm.BeginLink()
			return LastfmLinkStartedMsg{Request: id, Link: link, Err: err}
		}
	}
}

// completeLastfmLink asks the core to link the account that authorized
// token.
func completeLastfmLink(lfm *core.Lastfm, token string) func(request.ID) tea.Cmd {
	return func(id request.ID) tea.Cmd {
		return func() tea.Msg {
			username, err := lfm.CompleteLink(token)
			return LastfmLinkedMsg{Request: id, Username: username, Err: err}
		}
	}
}

// lastfmPopup returns the Last.fm popup, nil when it is closed.
func (m *Model) lastfmPopup() *lastfmauth.Model {
	lfm, _ := m.Popups.Get(popupctl.LastfmAuth).(*lastfmauth.Model) //nolint:errcheck // nil when closed
	return lfm
}

// handleLastfmMsg handles Last.fm related messages.
func (m *Model) handleLastfmMsg(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LastfmLinkStartedMsg:
		m.handleLastfmLinkStarted(msg)
	case LastfmLinkedMsg:
		m.handleLastfmLinked(msg)
	case lastfmauth.ActionMsg:
		return m.handleLastfmAuthAction(msg)
	}
	return *m, nil
}

// handleLastfmLinkStarted keeps the token of the link and sends the user to
// the browser to authorize it, if it is the link start waited for: one asked
// before another, or by a popup closed since, is dropped.
func (m *Model) handleLastfmLinkStarted(msg LastfmLinkStartedMsg) {
	if !m.lastfmLinkStart.Take(msg.Request) {
		return
	}
	pop := m.lastfmPopup()
	if pop == nil {
		return // closed meanwhile: nothing is linked yet, nothing to undo
	}
	if msg.Err != nil {
		pop.SetError(msg.Err.Error())
		return
	}
	m.lastfmAuthToken = msg.Link.Token
	pop.SetWaitingCallback()
	if m.openBrowser != nil {
		_ = m.openBrowser(msg.Link.URL) //nolint:errcheck // the popup tells what to do; the user can still open the URL
	}
}

// handleLastfmLinked shows the account the core linked, or why it did not.
// The Last.fm popup takes the completion it waits for. One it did not ask
// for, asked by a popup closed since, is never dropped: its failure is told
// by the error popup, the open popup left as it is; its success linked the
// account, which the open popup shows, the link it had in progress ended.
func (m *Model) handleLastfmLinked(msg LastfmLinkedMsg) {
	pop := m.lastfmPopup()
	taken := m.lastfmLinkCompletion.Take(msg.Request) && pop != nil
	switch {
	case taken && msg.Err != nil:
		pop.SetError(msg.Err.Error())
	case taken:
		pop.SetAccount(msg.Username, true)
	case msg.Err != nil:
		m.Popups.ShowOpError(errmsg.OpLastfmAuth, msg.Err)
	case pop != nil:
		m.lastfmAuthToken = ""
		m.lastfmLinkStart.Forget()
		m.lastfmLinkCompletion.Forget()
		pop.SetAccount(msg.Username, true)
	}
}

// handleLastfmAuthAction handles actions from the Last.fm auth popup.
func (m *Model) handleLastfmAuthAction(msg lastfmauth.ActionMsg) (Model, tea.Cmd) {
	lfm := m.Core.Lastfm()
	switch msg.Action { //nolint:exhaustive // ActionNone requires no handling
	case lastfmauth.ActionClose:
		m.lastfmAuthToken = "" // Clear pending token
		m.lastfmLinkStart.Forget()
		m.lastfmLinkCompletion.Forget()
		m.Popups.Hide(popupctl.LastfmAuth)

	case lastfmauth.ActionStartAuth:
		return *m, m.lastfmLinkStart.Ask(beginLastfmLink(lfm))

	case lastfmauth.ActionUnlink:
		if err := lfm.Unlink(); err != nil {
			m.Popups.ShowOpError(errmsg.OpLastfmAuth, err)
			return *m, nil
		}
		if pop := m.lastfmPopup(); pop != nil {
			pop.SetAccount("", false)
		}

	case lastfmauth.ActionConfirmAuth:
		// The user says they authorized the token the core was given.
		if m.lastfmAuthToken != "" {
			token := m.lastfmAuthToken
			m.lastfmAuthToken = ""
			return *m, m.lastfmLinkCompletion.Ask(completeLastfmLink(lfm, token))
		}
	}

	return *m, nil
}
