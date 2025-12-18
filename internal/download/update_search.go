package download

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/popup"
)

// handleSearchPhaseKey handles keyboard input in the search phase.
func (m *Model) handleSearchPhaseKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	// In StateSearch, only handle Enter - let other keys go to textinput
	if m.state == StateSearch {
		if msg.String() == keyEnter {
			cmd := m.handleSearchEnter()
			return m, cmd
		}
		// Return nil to let the key fall through to textinput
		return nil, nil
	}

	// In other search phase states (ArtistResults, ArtistSearching)
	switch msg.String() {
	case keyEnter:
		cmd := m.handleSearchEnter()
		return m, cmd
	case keyUp, "k":
		if m.state == StateArtistResults {
			m.artistCursor.Move(-1, len(m.artistResults), m.Height()-12)
		}
	case keyDown, "j":
		if m.state == StateArtistResults {
			m.artistCursor.Move(1, len(m.artistResults), m.Height()-12)
		}
	case keyHome, "g":
		if m.state == StateArtistResults {
			m.artistCursor.JumpStart()
		}
	case keyEnd, "G":
		if m.state == StateArtistResults {
			m.artistCursor.JumpEnd(len(m.artistResults), m.Height()-12)
		}
	case keyBackspace:
		m.handleSearchBack()
	}
	return m, nil
}

// handleSearchEnter processes Enter key in the search phase.
func (m *Model) handleSearchEnter() tea.Cmd {
	//nolint:exhaustive // Only handling search phase states
	switch m.state {
	case StateSearch:
		query := m.searchInput.Value()
		if query == "" {
			return nil
		}
		m.searchQuery = query
		m.state = StateArtistSearching
		m.statusMsg = "Searching artists..."
		m.errorMsg = ""
		return searchArtistsCmd(m.mbClient, query)

	case StateArtistResults:
		pos := m.artistCursor.Pos()
		if len(m.artistResults) == 0 || pos >= len(m.artistResults) {
			return nil
		}
		selected := m.artistResults[pos]
		m.selectedArtist = &selected
		m.state = StateReleaseGroupLoading
		m.statusMsg = "Loading releases..."
		return fetchReleaseGroupsCmd(m.mbClient, selected.ID)

	case StateArtistSearching:
		// No action during loading
		return nil

	default:
		// Other states handled by different phase handlers
		return nil
	}
}

// handleSearchBack processes Backspace key in the search phase.
func (m *Model) handleSearchBack() {
	//nolint:exhaustive // Only handling search phase states
	switch m.state {
	case StateSearch:
		// Let backspace be handled by text input (delete character)
	case StateArtistResults:
		// Go back to search
		m.state = StateSearch
		m.searchInput.Focus()
		m.artistResults = nil
		m.artistCursor.Reset()
		m.statusMsg = ""
		m.errorMsg = ""
	case StateArtistSearching:
		// No escape action in loading state
	default:
		// Other states handled by different phase handlers
	}
}

// handleArtistSearchResult processes artist search results.
func (m *Model) handleArtistSearchResult(msg ArtistSearchResultMsg) (popup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.state = StateSearch
		m.errorMsg = fmt.Sprintf("Search error: %v", msg.Err)
		m.statusMsg = ""
		m.searchInput.Focus()
		return m, nil
	}

	m.artistResults = msg.Artists
	m.artistCursor.Reset()
	m.state = StateArtistResults
	m.statusMsg = ""

	if len(m.artistResults) == 0 {
		m.statusMsg = "No artists found"
	}

	return m, nil
}
