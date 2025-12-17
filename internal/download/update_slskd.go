package download

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/popup"
)

// handleSlskdPhaseKey handles keyboard input in the slskd phase.
func (m *Model) handleSlskdPhaseKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case keyEnter:
		cmd := m.handleSlskdEnter()
		return m, cmd
	case keyUp, "k":
		if m.state == StateSlskdResults {
			m.slskdCursor.Move(-1, len(m.slskdResults), m.height-12)
		}
	case keyDown, "j":
		if m.state == StateSlskdResults {
			m.slskdCursor.Move(1, len(m.slskdResults), m.height-12)
		}
	case keyHome, "g":
		if m.state == StateSlskdResults {
			m.slskdCursor.JumpStart()
		}
	case keyEnd, "G":
		if m.state == StateSlskdResults {
			m.slskdCursor.JumpEnd(len(m.slskdResults), m.height-12)
		}
	case keyBackspace:
		m.handleSlskdBack()
	case "f":
		// Cycle format filter
		if m.state == StateSlskdResults {
			m.cycleFormatFilter()
		}
	case "s":
		// Toggle no slot filter
		if m.state == StateSlskdResults {
			m.filterNoSlot = !m.filterNoSlot
			m.reapplyFilters()
		}
	case "t":
		// Toggle track count filter
		if m.state == StateSlskdResults {
			m.filterTrackCount = !m.filterTrackCount
			m.reapplyFilters()
		}
	}
	return m, nil
}

// handleSlskdEnter processes Enter key in the slskd phase.
func (m *Model) handleSlskdEnter() tea.Cmd {
	// If download is complete, close the popup
	if m.downloadComplete {
		return func() tea.Msg { return CloseMsg{} }
	}

	//nolint:exhaustive // Only handling slskd phase states
	switch m.state {
	case StateSlskdResults:
		pos := m.slskdCursor.Pos()
		if len(m.slskdResults) == 0 || pos >= len(m.slskdResults) {
			return nil
		}
		selected := m.slskdResults[pos]
		m.state = StateDownloading
		m.statusMsg = "Queueing download..."
		return queueDownloadCmd(m.slskdClient, selected)

	case StateSlskdSearching, StateDownloading:
		// No action during loading
		return nil

	default:
		// Other states handled by different phase handlers
		return nil
	}
}

// handleSlskdBack processes Backspace key in the slskd phase.
func (m *Model) handleSlskdBack() {
	//nolint:exhaustive // Only handling slskd phase states
	switch m.state {
	case StateSlskdResults:
		// Go back to release results (or release groups if auto-selected)
		if len(m.releases) > 0 {
			m.state = StateReleaseResults
		} else {
			m.state = StateReleaseGroupResults
			m.selectedReleaseGroup = nil
		}
		m.slskdRawResponse = nil
		m.slskdResults = nil
		m.slskdCursor.Reset()
		m.statusMsg = ""
	case StateDownloading:
		// Go back to slskd results
		m.state = StateSlskdResults
		m.statusMsg = ""
	case StateSlskdSearching:
		// No escape action in loading state
	default:
		// Other states handled by different phase handlers
	}
}

// cycleFormatFilter cycles through format filter options.
func (m *Model) cycleFormatFilter() {
	switch m.formatFilter {
	case FormatBoth:
		m.formatFilter = FormatLossless
	case FormatLossless:
		m.formatFilter = FormatLossy
	case FormatLossy:
		m.formatFilter = FormatBoth
	}
	m.reapplyFilters()
}

// startSlskdSearchWithTrackCount starts slskd search with the expected track count set.
func (m *Model) startSlskdSearchWithTrackCount() tea.Cmd {
	m.state = StateSlskdSearching
	m.statusMsg = "Searching slskd..."
	query := fmt.Sprintf("%s %s", m.selectedArtist.Name, m.selectedReleaseGroup.Title)
	return startSlskdSearchCmd(m.slskdClient, query)
}

// handleSlskdSearchStarted processes slskd search initiation.
func (m *Model) handleSlskdSearchStarted(msg SlskdSearchStartedMsg) (popup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.state = StateReleaseGroupResults
		m.errorMsg = fmt.Sprintf("slskd error: %v", msg.Err)
		m.statusMsg = ""
		return m, nil
	}

	m.slskdSearchID = msg.SearchID
	// Start polling for results
	return m, scheduleSlskdPollCmd(msg.SearchID)
}

// handleSlskdSearchResult processes slskd search results.
func (m *Model) handleSlskdSearchResult(msg SlskdSearchResultMsg) (popup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.state = StateReleaseGroupResults
		m.errorMsg = fmt.Sprintf("slskd error: %v", msg.Err)
		m.statusMsg = ""
		return m, nil
	}

	// Store raw responses for re-filtering
	m.slskdRawResponse = msg.RawResponses
	m.slskdCursor.Reset()
	m.state = StateSlskdResults
	m.statusMsg = ""

	// Apply current filters
	m.reapplyFilters()

	if len(m.slskdResults) == 0 {
		m.statusMsg = "No matching results found"
	}

	return m, nil
}

// handleDownloadQueued processes download queue confirmation.
func (m *Model) handleDownloadQueued(msg SlskdDownloadQueuedMsg) (popup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.state = StateSlskdResults
		m.errorMsg = fmt.Sprintf("Download error: %v", msg.Err)
		m.statusMsg = ""
		return m, nil
	}

	// Capture data before reset for persistence
	var dataMsg *QueuedDataMsg
	pos := m.slskdCursor.Pos()
	if m.selectedArtist != nil && m.selectedReleaseGroup != nil &&
		pos < len(m.slskdResults) {
		selected := m.slskdResults[pos]

		files := make([]FileInfo, len(selected.Files))
		for i, f := range selected.Files {
			files[i] = FileInfo{
				Filename: f.Filename,
				Size:     f.Size,
			}
		}

		// Get release ID if a specific release was selected
		var releaseID string
		if m.selectedRelease != nil {
			releaseID = m.selectedRelease.ID
		}

		dataMsg = &QueuedDataMsg{
			MBReleaseGroupID: m.selectedReleaseGroup.ID,
			MBReleaseID:      releaseID,
			MBArtistName:     m.selectedArtist.Name,
			MBAlbumTitle:     m.selectedReleaseGroup.Title,
			MBReleaseYear:    m.selectedReleaseGroup.FirstRelease,
			SlskdUsername:    selected.Username,
			SlskdDirectory:   selected.Directory,
			Files:            files,
			// Full MusicBrainz data for importing
			MBReleaseGroup:   m.selectedReleaseGroup,
			MBReleaseDetails: m.selectedReleaseDetails,
		}
	}

	// Mark download complete - popup can now be closed with Enter/Esc
	m.Reset()
	m.downloadComplete = true
	m.statusMsg = "Download queued successfully! Press Enter or Esc to close."

	// Emit the data message for the app to persist
	if dataMsg != nil {
		return m, func() tea.Msg { return *dataMsg }
	}
	return m, nil
}

// updateSlskdPollStatus updates the status message based on poll state.
func (m *Model) updateSlskdPollStatus(state string, responseCount, stablePolls, fetchRetries, totalPolls int) {
	switch {
	case fetchRetries > 0:
		m.statusMsg = fmt.Sprintf("Waiting for results... (%d users, attempt %d)", responseCount, fetchRetries)
	case stablePolls > 0:
		m.statusMsg = fmt.Sprintf("Collecting results... (%d users responded)", responseCount)
	default:
		stateInfo := "searching"
		if state != "" && state != "InProgress" {
			stateInfo = state
		}
		// Show elapsed time in status when polling for a while
		elapsed := totalPolls / 2 // Each poll is ~500ms
		if elapsed > 10 {
			m.statusMsg = fmt.Sprintf("Searching Soulseek (%s) - %d users (%ds)", stateInfo, responseCount, elapsed)
		} else {
			m.statusMsg = fmt.Sprintf("Searching Soulseek (%s) - %d users responded", stateInfo, responseCount)
		}
	}
}
