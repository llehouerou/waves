package download

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/musicbrainz/workflow"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// Compile-time check that Model implements popup.Popup.
var _ popup.Popup = (*Model)(nil)

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	var cmds []tea.Cmd

	// Track if we were in search state before handling keys
	wasInSearch := m.state == StateSearch

	switch msg := msg.(type) {
	case tea.KeyMsg:
		newModel, cmd := m.handleKey(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if newModel != nil {
			return newModel, tea.Batch(cmds...)
		}

	case workflow.ArtistSearchResultMsg:
		return m.handleArtistSearchResult(msg)

	case workflow.SearchResultMsg:
		return m.handleReleaseGroupResult(msg)

	case workflow.ReleasesResultMsg:
		return m.handleReleaseResult(msg)

	case workflow.ReleaseDetailsResultMsg:
		return m.handleReleaseDetailsResult(msg)

	case SlskdSearchStartedMsg:
		return m.handleSlskdSearchStarted(msg)

	case SlskdSearchPollMsg:
		// Update status message before polling
		m.updateSlskdPollStatus(msg.State, msg.ResponseCount, msg.StablePolls, msg.FetchRetries, msg.TotalPolls)
		return m, pollSlskdSearchCmd(slskdPollParams{
			client:            m.slskdClient,
			searchID:          msg.SearchID,
			lastResponseCount: msg.ResponseCount,
			stablePolls:       msg.StablePolls,
			fetchRetries:      msg.FetchRetries,
			totalPolls:        msg.TotalPolls,
		})

	case SlskdPollContinueMsg:
		// Update status and schedule next poll with delay
		m.updateSlskdPollStatus(msg.State, msg.ResponseCount, msg.StablePolls, msg.FetchRetries, msg.TotalPolls)
		return m, scheduleSlskdPollWithStateCmd(msg)

	case SlskdSearchResultMsg:
		return m.handleSlskdSearchResult(msg)

	case SlskdDownloadQueuedMsg:
		return m.handleDownloadQueued(msg)
	}

	// Update text input only if we were already in search state
	// (prevents backspace from deleting text when navigating back)
	if wasInSearch {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// handleKey processes keyboard input and routes to phase-specific handlers.
func (m *Model) handleKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	// Esc always closes the popup
	if msg.String() == "esc" {
		return m, func() tea.Msg { return ActionMsg(Close{}) }
	}

	// Route to phase-specific handler
	switch {
	case m.state.IsSearchPhase():
		return m.handleSearchPhaseKey(msg)
	case m.state.IsReleaseGroupPhase():
		return m.handleReleaseGroupPhaseKey(msg)
	case m.state.IsReleasePhase():
		return m.handleReleasePhaseKey(msg)
	case m.state.IsSlskdPhase():
		return m.handleSlskdPhaseKey(msg)
	}

	return nil, nil
}

// Init initializes the download view.
func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}
