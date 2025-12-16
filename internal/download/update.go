package download

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/musicbrainz"
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
		cmd := m.handleKey(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case ArtistSearchResultMsg:
		return m.handleArtistSearchResult(msg)

	case ReleaseGroupResultMsg:
		return m.handleReleaseGroupResult(msg)

	case ReleaseResultMsg:
		return m.handleReleaseResult(msg)

	case ReleaseDetailsResultMsg:
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

// handleKey processes keyboard input.
func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		return m.handleEnter()
	case "esc":
		// Esc always closes the popup
		return func() tea.Msg { return CloseMsg{} }
	case "backspace":
		// Backspace goes back a step (except in search input mode)
		return m.handleBack()
	case "up", "k":
		m.moveCursorUp()
	case "down", "j":
		m.moveCursorDown()
	case "home", "g":
		m.moveCursorToStart()
	case "end", "G":
		m.moveCursorToEnd()
	case "f":
		// Cycle format filter (only in slskd results)
		if m.state == StateSlskdResults {
			m.cycleFormatFilter()
		}
	case "s":
		// Toggle no slot filter (only in slskd results)
		if m.state == StateSlskdResults {
			m.filterNoSlot = !m.filterNoSlot
			m.reapplyFilters()
		}
	case "t":
		// Toggle track count filter (only in slskd results)
		if m.state == StateSlskdResults {
			m.filterTrackCount = !m.filterTrackCount
			m.reapplyFilters()
		}
	case "a":
		// Toggle albums only filter (only in release group results)
		if m.state == StateReleaseGroupResults {
			m.albumsOnly = !m.albumsOnly
			m.reapplyReleaseGroupFilters()
		}
	case "d":
		// Toggle deduplicate filter (only in release results)
		if m.state == StateReleaseResults {
			m.deduplicateRelease = !m.deduplicateRelease
			m.reapplyReleaseFilters()
		}
	}
	return nil
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

// handleEnter processes the Enter key based on current state.
func (m *Model) handleEnter() tea.Cmd {
	// If download is complete, close the popup
	if m.downloadComplete {
		return func() tea.Msg { return CloseMsg{} }
	}

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
		if len(m.artistResults) == 0 || m.artistCursor >= len(m.artistResults) {
			return nil
		}
		selected := m.artistResults[m.artistCursor]
		m.selectedArtist = &selected
		m.state = StateReleaseGroupLoading
		m.statusMsg = "Loading releases..."
		return fetchReleaseGroupsCmd(m.mbClient, selected.ID)

	case StateReleaseGroupResults:
		if len(m.releaseGroups) == 0 || m.releaseGroupCursor >= len(m.releaseGroups) {
			return nil
		}
		selected := m.releaseGroups[m.releaseGroupCursor]
		m.selectedReleaseGroup = &selected
		m.state = StateReleaseLoading
		m.statusMsg = "Loading track info..."
		return fetchReleasesCmd(m.mbClient, selected.ID)

	case StateReleaseResults:
		if len(m.releases) == 0 || m.releaseCursor >= len(m.releases) {
			return nil
		}
		// User selected a release - fetch full details with tracks
		selected := m.releases[m.releaseCursor]
		m.selectedRelease = &selected
		m.expectedTracks = selected.TrackCount
		m.state = StateReleaseDetailsLoading
		m.statusMsg = "Loading release details..."
		return fetchReleaseDetailsCmd(m.mbClient, selected.ID)

	case StateSlskdResults:
		if len(m.slskdResults) == 0 || m.slskdCursor >= len(m.slskdResults) {
			return nil
		}
		selected := m.slskdResults[m.slskdCursor]
		m.state = StateDownloading
		m.statusMsg = "Queueing download..."
		return queueDownloadCmd(m.slskdClient, selected)

	case StateArtistSearching, StateReleaseGroupLoading, StateReleaseLoading,
		StateReleaseDetailsLoading, StateSlskdSearching, StateDownloading:
		// No action during loading states
		return nil
	}

	return nil
}

// handleBack processes the Backspace key to go back a step.
// In StateSearch, backspace is handled by the text input, so this is a no-op.
func (m *Model) handleBack() tea.Cmd {
	switch m.state {
	case StateSearch:
		// Let backspace be handled by text input (delete character)
		return nil
	case StateArtistResults:
		// Go back to search
		m.state = StateSearch
		m.searchInput.Focus()
		m.artistResults = nil
		m.artistCursor = 0
		m.statusMsg = ""
		m.errorMsg = ""
	case StateReleaseGroupResults:
		// Go back to artist results
		m.state = StateArtistResults
		m.selectedArtist = nil
		m.releaseGroupsRaw = nil
		m.releaseGroups = nil
		m.releaseGroupCursor = 0
		m.statusMsg = ""
	case StateReleaseResults:
		// Go back to release groups
		m.state = StateReleaseGroupResults
		m.selectedReleaseGroup = nil
		m.releases = nil
		m.releaseCursor = 0
		m.expectedTracks = 0
		m.statusMsg = ""
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
		m.slskdCursor = 0
		m.statusMsg = ""
	case StateDownloading:
		// Go back to slskd results
		m.state = StateSlskdResults
		m.statusMsg = ""
	case StateArtistSearching, StateReleaseGroupLoading, StateReleaseLoading,
		StateReleaseDetailsLoading, StateSlskdSearching:
		// No escape action in these loading states
	}
	return nil
}

// moveCursorUp moves the cursor up in the current list.
func (m *Model) moveCursorUp() {
	if cursor := m.currentCursor(); cursor != nil && *cursor > 0 {
		*cursor--
	}
}

// moveCursorDown moves the cursor down in the current list.
func (m *Model) moveCursorDown() {
	if cursor := m.currentCursor(); cursor != nil {
		maxIdx := m.currentListLen() - 1
		if *cursor < maxIdx {
			*cursor++
		}
	}
}

// moveCursorToStart moves cursor to the start of the list.
func (m *Model) moveCursorToStart() {
	if cursor := m.currentCursor(); cursor != nil {
		*cursor = 0
	}
}

// moveCursorToEnd moves cursor to the end of the list.
func (m *Model) moveCursorToEnd() {
	if cursor := m.currentCursor(); cursor != nil {
		if length := m.currentListLen(); length > 0 {
			*cursor = length - 1
		}
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
	m.artistCursor = 0
	m.state = StateArtistResults
	m.statusMsg = ""

	if len(m.artistResults) == 0 {
		m.statusMsg = "No artists found"
	}

	return m, nil
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

	m.releaseGroupCursor = 0
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
	// Reset cursor if out of bounds
	if m.releaseGroupCursor >= len(m.releaseGroups) {
		m.releaseGroupCursor = max(0, len(m.releaseGroups)-1)
	}
}

// handleReleaseResult processes release results for track count determination.
func (m *Model) handleReleaseResult(msg ReleaseResultMsg) (popup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.state = StateReleaseGroupResults
		m.errorMsg = fmt.Sprintf("Error loading releases: %v", msg.Err)
		m.statusMsg = ""
		return m, nil
	}

	// Handle case where no releases found
	if len(msg.Releases) == 0 {
		m.state = StateReleaseGroupResults
		m.errorMsg = "No releases found for this release group"
		m.statusMsg = ""
		return m, nil
	}

	// Sort releases by date ascending (oldest first)
	releases := msg.Releases
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].Date < releases[j].Date
	})

	// Store raw releases for re-filtering
	m.releasesRaw = releases
	m.reapplyReleaseFilters()

	// Always show release selection - user must choose for import process
	m.releaseCursor = 0
	m.state = StateReleaseResults
	m.statusMsg = "Select a release"
	return m, nil
}

// handleReleaseDetailsResult processes release details and starts slskd search.
func (m *Model) handleReleaseDetailsResult(msg ReleaseDetailsResultMsg) (popup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.state = StateReleaseResults
		m.errorMsg = fmt.Sprintf("Error loading release details: %v", msg.Err)
		m.statusMsg = ""
		return m, nil
	}

	// Store the full release details for later use (importing)
	m.selectedReleaseDetails = msg.Details

	// Now start slskd search
	cmd := m.startSlskdSearchWithTrackCount()
	return m, cmd
}

// reapplyReleaseFilters filters releases based on current deduplicateRelease setting.
func (m *Model) reapplyReleaseFilters() {
	if m.deduplicateRelease {
		m.releases = deduplicateReleases(m.releasesRaw)
	} else {
		m.releases = m.releasesRaw
	}
	// Reset cursor if out of bounds
	if m.releaseCursor >= len(m.releases) {
		m.releaseCursor = max(0, len(m.releases)-1)
	}
}

// releaseKey is used for deduplicating releases.
type releaseKey struct {
	trackCount  int
	year        string
	formats     string
	releaseType string
}

// deduplicateReleases removes duplicate releases based on (track count, year, formats, release type).
// For duplicates, keeps the one with preferred country: XW > US > others.
// Preserves the original order (assumes already sorted by date).
func deduplicateReleases(releases []musicbrainz.Release) []musicbrainz.Release {
	seen := make(map[releaseKey]int) // key -> index in result
	result := make([]musicbrainz.Release, 0, len(releases))

	for i := range releases {
		r := &releases[i]
		key := releaseKey{
			trackCount:  r.TrackCount,
			year:        extractYear(r.Date),
			formats:     r.Formats,
			releaseType: r.ReleaseType,
		}

		if existingIdx, exists := seen[key]; exists {
			// Compare countries - replace if new one is better
			existing := &result[existingIdx]
			if countryPriority(r.Country) < countryPriority(existing.Country) {
				result[existingIdx] = *r
			}
		} else {
			// New key - add to result
			seen[key] = len(result)
			result = append(result, *r)
		}
	}

	return result
}

// countryPriority returns priority for country (lower is better).
// XW (worldwide) > US > others.
func countryPriority(country string) int {
	switch country {
	case "XW":
		return 0
	case "US":
		return 1
	default:
		return 2
	}
}

// extractYear extracts the year (first 4 chars) from a date string.
func extractYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return date
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
	m.slskdCursor = 0
	m.state = StateSlskdResults
	m.statusMsg = ""

	// Apply current filters
	m.reapplyFilters()

	if len(m.slskdResults) == 0 {
		m.statusMsg = "No matching results found"
	}

	// TODO: Re-enable cleanup after debugging
	// if m.slskdSearchID != "" {
	// 	go func() {
	// 		_ = m.slskdClient.DeleteSearch(m.slskdSearchID)
	// 	}()
	// }

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
	if m.selectedArtist != nil && m.selectedReleaseGroup != nil &&
		m.slskdCursor < len(m.slskdResults) {
		selected := m.slskdResults[m.slskdCursor]

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

// Init initializes the download view.
func (m *Model) Init() tea.Cmd {
	return textinput.Blink
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
