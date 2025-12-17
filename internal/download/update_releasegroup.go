package download

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// handleReleaseGroupPhaseKey handles keyboard input in the release group phase.
func (m *Model) handleReleaseGroupPhaseKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case keyEnter:
		cmd := m.handleReleaseGroupEnter()
		return m, cmd
	case keyUp, "k":
		if m.state == StateReleaseGroupResults {
			m.releaseGroupCursor.Move(-1, len(m.releaseGroups), m.height-12)
		}
	case keyDown, "j":
		if m.state == StateReleaseGroupResults {
			m.releaseGroupCursor.Move(1, len(m.releaseGroups), m.height-12)
		}
	case keyHome, "g":
		if m.state == StateReleaseGroupResults {
			m.releaseGroupCursor.JumpStart()
		}
	case keyEnd, "G":
		if m.state == StateReleaseGroupResults {
			m.releaseGroupCursor.JumpEnd(len(m.releaseGroups), m.height-12)
		}
	case keyBackspace:
		m.handleReleaseGroupBack()
	case "a":
		// Toggle albums only filter
		if m.state == StateReleaseGroupResults {
			m.albumsOnly = !m.albumsOnly
			m.reapplyReleaseGroupFilters()
		}
	}
	return m, nil
}

// handleReleaseGroupEnter processes Enter key in the release group phase.
func (m *Model) handleReleaseGroupEnter() tea.Cmd {
	//nolint:exhaustive // Only handling release group phase states
	switch m.state {
	case StateReleaseGroupResults:
		pos := m.releaseGroupCursor.Pos()
		if len(m.releaseGroups) == 0 || pos >= len(m.releaseGroups) {
			return nil
		}
		selected := m.releaseGroups[pos]
		m.selectedReleaseGroup = &selected
		m.state = StateReleaseLoading
		m.statusMsg = "Loading track info..."
		return fetchReleasesCmd(m.mbClient, selected.ID)

	case StateReleaseGroupLoading:
		// No action during loading
		return nil

	default:
		// Other states handled by different phase handlers
		return nil
	}
}

// handleReleaseGroupBack processes Backspace key in the release group phase.
func (m *Model) handleReleaseGroupBack() {
	//nolint:exhaustive // Only handling release group phase states
	switch m.state {
	case StateReleaseGroupResults:
		// Go back to artist results
		m.state = StateArtistResults
		m.selectedArtist = nil
		m.releaseGroupsRaw = nil
		m.releaseGroups = nil
		m.releaseGroupCursor.Reset()
		m.statusMsg = ""
	case StateReleaseGroupLoading:
		// No escape action in loading state
	default:
		// Other states handled by different phase handlers
	}
}

// handleReleaseGroupResult processes release group results.
func (m *Model) handleReleaseGroupResult(msg ReleaseGroupResultMsg) (popup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.state = StateArtistResults
		m.errorMsg = fmt.Sprintf("Error loading releases: %v", msg.Err)
		m.statusMsg = ""
		return m, nil
	}

	// Store raw results for re-filtering
	m.releaseGroupsRaw = msg.ReleaseGroups
	m.reapplyReleaseGroupFilters()

	m.releaseGroupCursor.Reset()
	m.state = StateReleaseGroupResults
	m.statusMsg = ""

	if len(m.releaseGroups) == 0 {
		m.statusMsg = "No releases found"
	}

	return m, nil
}

// reapplyReleaseGroupFilters filters release groups based on current albumsOnly setting.
func (m *Model) reapplyReleaseGroupFilters() {
	if m.albumsOnly {
		filtered := make([]musicbrainz.ReleaseGroup, 0, len(m.releaseGroupsRaw))
		for i := range m.releaseGroupsRaw {
			rg := &m.releaseGroupsRaw[i]
			if rg.PrimaryType == "Album" && !hasExcludedSecondaryType(rg.SecondaryTypes) {
				filtered = append(filtered, *rg)
			}
		}
		m.releaseGroups = filtered
	} else {
		m.releaseGroups = m.releaseGroupsRaw
	}
	// Clamp cursor if out of bounds
	m.releaseGroupCursor.ClampToBounds(len(m.releaseGroups))
}

// hasExcludedSecondaryType checks if a release group has secondary types we want to filter out.
func hasExcludedSecondaryType(secondaryTypes []string) bool {
	excluded := map[string]bool{
		"Live":        true,
		"Compilation": true,
	}
	for _, t := range secondaryTypes {
		if excluded[t] {
			return true
		}
	}
	return false
}
