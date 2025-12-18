package retag

import (
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/importer"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/player"
	uipopup "github.com/llehouerou/waves/internal/ui/popup"
)

const (
	emptyPlaceholder = "(empty)"
)

// Compile-time check that Model implements popup.Popup.
var _ uipopup.Popup = (*Model)(nil)

// Init initializes the retag popup and starts reading tags.
func (m *Model) Init() tea.Cmd {
	return ReadAlbumTagsCmd(m.trackPaths)
}

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (uipopup.Popup, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case TagsReadMsg:
		return m.handleTagsRead(msg)
	case ReleaseGroupSearchResultMsg:
		return m.handleReleaseGroupSearchResult(msg)
	case ReleasesFetchedMsg:
		return m.handleReleasesFetched(msg)
	case ReleaseDetailsFetchedMsg:
		return m.handleReleaseDetailsFetched(msg)
	case FileRetaggedMsg:
		return m.handleFileRetagged(msg)
	case LibraryUpdatedMsg:
		return m.handleLibraryUpdated(msg)
	}
	return m, nil
}

// handleKey handles key presses based on current state.
func (m *Model) handleKey(msg tea.KeyMsg) (uipopup.Popup, tea.Cmd) {
	key := msg.String()

	// Handle search input mode first
	if m.searchMode {
		return m.handleSearchInput(msg)
	}

	switch key {
	case "esc":
		return m.handleEscape()
	case "enter":
		return m.handleEnter()
	case "j", "down":
		return m.handleDown()
	case "k", "up":
		return m.handleUp()
	case "backspace":
		return m.handleBackspace()
	case "/":
		return m.handleSearchToggle()
	}

	return m, nil
}

// handleSearchInput handles key presses in search mode.
func (m *Model) handleSearchInput(msg tea.KeyMsg) (uipopup.Popup, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		// Exit search mode without searching
		m.searchMode = false
		m.searchInput.Blur()
		return m, nil
	case "enter":
		// Execute search
		m.searchMode = false
		m.searchInput.Blur()
		query := m.searchInput.Value()
		if query == "" {
			query = m.initialSearch
		}
		m.state = StateSearching
		m.statusMsg = "Searching MusicBrainz..."
		return m, SearchReleaseGroupsCmd(m.mbClient, query)
	default:
		// Update text input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}
}

// handleEscape handles the escape key.
func (m *Model) handleEscape() (uipopup.Popup, tea.Cmd) {
	switch m.state {
	case StateLoading, StateSearching, StateReleaseLoading, StateReleaseDetailsLoading:
		// Close during loading states
		return m, func() tea.Msg { return ActionMsg(Close{}) }
	case StateReleaseGroupResults, StateReleaseResults, StateTagPreview:
		// Close popup
		return m, func() tea.Msg { return ActionMsg(Close{}) }
	case StateRetagging:
		// Allow closing during retag (retag continues in background)
		return m, func() tea.Msg { return ActionMsg(Close{}) }
	case StateComplete:
		// Close and signal completion
		return m, func() tea.Msg {
			return ActionMsg(Complete{
				AlbumArtist:  m.albumArtist,
				AlbumName:    m.albumName,
				SuccessCount: m.successCount,
				FailedCount:  len(m.failedFiles),
			})
		}
	}
	return m, nil
}

// handleEnter handles the enter key.
func (m *Model) handleEnter() (uipopup.Popup, tea.Cmd) {
	switch m.state { //nolint:exhaustive // only handle selectable states
	case StateReleaseGroupResults:
		// Select release group and load releases
		if len(m.releaseGroups) == 0 {
			return m, nil
		}
		idx := m.releaseGroupCursor.Pos()
		if idx >= len(m.releaseGroups) {
			return m, nil
		}
		m.selectedReleaseGroup = &m.releaseGroups[idx]
		m.state = StateReleaseLoading
		m.statusMsg = "Loading releases..."
		return m, FetchReleasesCmd(m.mbClient, m.selectedReleaseGroup.ID)

	case StateReleaseResults:
		// Select release and load full details
		if len(m.releases) == 0 {
			return m, nil
		}
		idx := m.releaseCursor.Pos()
		if idx >= len(m.releases) {
			return m, nil
		}
		release := m.releases[idx]
		m.state = StateReleaseDetailsLoading
		m.statusMsg = "Loading release details..."
		return m, FetchReleaseDetailsCmd(m.mbClient, release.ID)

	case StateTagPreview:
		// Start retagging
		m.state = StateRetagging
		cmd := m.startRetag()
		return m, cmd

	case StateComplete:
		// Close and signal completion
		return m, func() tea.Msg {
			return ActionMsg(Complete{
				AlbumArtist:  m.albumArtist,
				AlbumName:    m.albumName,
				SuccessCount: m.successCount,
				FailedCount:  len(m.failedFiles),
			})
		}
	}
	return m, nil
}

// handleDown handles down/j key.
func (m *Model) handleDown() (uipopup.Popup, tea.Cmd) {
	maxVisible := max(m.height-12, 5)
	switch m.state { //nolint:exhaustive // only handle list states
	case StateReleaseGroupResults:
		m.releaseGroupCursor.Move(1, len(m.releaseGroups), maxVisible)
	case StateReleaseResults:
		m.releaseCursor.Move(1, len(m.releases), maxVisible)
	}
	return m, nil
}

// handleUp handles up/k key.
func (m *Model) handleUp() (uipopup.Popup, tea.Cmd) {
	maxVisible := max(m.height-12, 5)
	switch m.state { //nolint:exhaustive // only handle list states
	case StateReleaseGroupResults:
		m.releaseGroupCursor.Move(-1, len(m.releaseGroups), maxVisible)
	case StateReleaseResults:
		m.releaseCursor.Move(-1, len(m.releases), maxVisible)
	}
	return m, nil
}

// handleBackspace handles the backspace key to go back.
func (m *Model) handleBackspace() (uipopup.Popup, tea.Cmd) {
	switch m.state { //nolint:exhaustive // only handle states with back navigation
	case StateReleaseResults:
		// Go back to release group selection
		m.state = StateReleaseGroupResults
		m.releases = nil
		m.releaseCursor.Reset()
	case StateTagPreview:
		// Go back to release selection
		m.state = StateReleaseResults
		m.releaseDetails = nil
		m.tagDiffs = nil
	}
	return m, nil
}

// handleSearchToggle toggles search mode for refinement.
func (m *Model) handleSearchToggle() (uipopup.Popup, tea.Cmd) {
	if m.state == StateReleaseGroupResults {
		m.searchMode = true
		m.searchInput.SetValue(m.initialSearch)
		m.searchInput.Focus()
		return m, nil
	}
	return m, nil
}

// handleTagsRead handles the result of reading tags from files.
func (m *Model) handleTagsRead(msg TagsReadMsg) (uipopup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.currentTags = make([]player.TrackInfo, len(m.trackPaths))
	} else {
		m.currentTags = msg.Tags
	}

	// Store found IDs for display
	m.foundMBReleaseID = msg.MBReleaseID
	m.foundMBReleaseGroupID = msg.MBReleaseGroupID
	m.foundMBArtistID = msg.MBArtistID

	m.state = StateSearching

	// Priority 1: If we found a MusicBrainz Release ID, fetch that release directly
	if msg.MBReleaseID != "" {
		m.searchMethod = "Found release ID in tags"
		m.statusMsg = "Loading release from MusicBrainz..."
		return m, FetchReleaseByIDCmd(m.mbClient, msg.MBReleaseID)
	}

	// Priority 2: If we found a Release Group ID, fetch releases for that group
	if msg.MBReleaseGroupID != "" {
		m.searchMethod = "Found release group ID in tags"
		m.statusMsg = "Loading releases for release group..."
		return m, FetchReleasesForReleaseGroupCmd(m.mbClient, msg.MBReleaseGroupID)
	}

	// Priority 3: If we found an Artist ID, browse their release groups
	if msg.MBArtistID != "" {
		m.searchMethod = "Browsing artist's releases (artist ID in tags)"
		m.statusMsg = "Loading artist's release groups..."
		return m, FetchReleaseGroupsByArtistIDCmd(m.mbClient, msg.MBArtistID)
	}

	// Priority 4: Search by artist name + album name
	m.searchMethod = "Searching by artist and album name"
	m.statusMsg = "Searching MusicBrainz..."
	return m, SearchReleaseGroupsCmd(m.mbClient, m.albumArtist+" "+m.albumName)
}

// handleReleaseGroupSearchResult handles release group search results.
func (m *Model) handleReleaseGroupSearchResult(msg ReleaseGroupSearchResultMsg) (uipopup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.errorMsg = msg.Err.Error()
		m.state = StateReleaseGroupResults
		return m, nil
	}

	m.releaseGroups = filterAlbumReleaseGroups(msg.ReleaseGroups)

	// Sort results: exact matches first, then by artist match, then album match
	m.sortReleaseGroupsByRelevance()

	// Check for unique exact match (both artist and album match)
	exactMatches := m.findExactMatches()
	if len(exactMatches) == 1 {
		// Auto-select the exact match
		m.selectedReleaseGroup = &m.releaseGroups[exactMatches[0]]
		m.state = StateReleaseLoading
		m.statusMsg = "Loading releases..."
		return m, FetchReleasesCmd(m.mbClient, m.selectedReleaseGroup.ID)
	}

	m.releaseGroupCursor.Reset()
	m.releaseGroupCursor.ClampToBounds(len(m.releaseGroups))
	m.state = StateReleaseGroupResults
	m.statusMsg = ""
	m.errorMsg = ""

	return m, nil
}

// sortReleaseGroupsByRelevance sorts release groups with best matches first.
func (m *Model) sortReleaseGroupsByRelevance() {
	sort.SliceStable(m.releaseGroups, func(i, j int) bool {
		scoreI := m.releaseGroupMatchScore(&m.releaseGroups[i])
		scoreJ := m.releaseGroupMatchScore(&m.releaseGroups[j])
		return scoreI > scoreJ
	})
}

// releaseGroupMatchScore returns a score for how well a release group matches.
// Higher score = better match.
func (m *Model) releaseGroupMatchScore(rg *musicbrainz.ReleaseGroup) int {
	score := 0

	// Exact artist match (case-insensitive)
	if strings.EqualFold(rg.Artist, m.albumArtist) {
		score += 100
	}

	// Exact album match (case-insensitive)
	if strings.EqualFold(rg.Title, m.albumName) {
		score += 50
	}

	return score
}

// findExactMatches returns indices of release groups with exact artist AND album match.
func (m *Model) findExactMatches() []int {
	var matches []int
	for i := range m.releaseGroups {
		rg := &m.releaseGroups[i]
		if strings.EqualFold(rg.Artist, m.albumArtist) && strings.EqualFold(rg.Title, m.albumName) {
			matches = append(matches, i)
		}
	}
	return matches
}

// handleReleasesFetched handles releases fetched for a release group.
func (m *Model) handleReleasesFetched(msg ReleasesFetchedMsg) (uipopup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.errorMsg = msg.Err.Error()
		m.state = StateReleaseResults
		return m, nil
	}

	m.releases = sortReleasesByDate(msg.Releases)

	// Sort releases with matching track count first
	m.sortReleasesByTrackCountMatch()

	// Find releases with matching track count
	matchingIndices := m.findReleasesWithMatchingTrackCount()
	if len(matchingIndices) == 1 {
		// Auto-select the only release with matching track count
		release := m.releases[matchingIndices[0]]
		m.state = StateReleaseDetailsLoading
		m.statusMsg = "Loading release details..."
		return m, FetchReleaseDetailsCmd(m.mbClient, release.ID)
	}

	m.releaseCursor.Reset()
	m.releaseCursor.ClampToBounds(len(m.releases))
	m.state = StateReleaseResults
	m.statusMsg = ""
	m.errorMsg = ""

	return m, nil
}

// sortReleasesByTrackCountMatch sorts releases with matching track count first.
func (m *Model) sortReleasesByTrackCountMatch() {
	localTrackCount := len(m.trackPaths)
	sort.SliceStable(m.releases, func(i, j int) bool {
		iMatches := m.releases[i].TrackCount == localTrackCount
		jMatches := m.releases[j].TrackCount == localTrackCount
		if iMatches != jMatches {
			return iMatches // Matching comes first
		}
		return false // Keep original order (by date)
	})
}

// findReleasesWithMatchingTrackCount returns indices of releases with matching track count.
func (m *Model) findReleasesWithMatchingTrackCount() []int {
	localTrackCount := len(m.trackPaths)
	var matches []int
	for i := range m.releases {
		if m.releases[i].TrackCount == localTrackCount {
			matches = append(matches, i)
		}
	}
	return matches
}

// handleReleaseDetailsFetched handles full release details.
func (m *Model) handleReleaseDetailsFetched(msg ReleaseDetailsFetchedMsg) (uipopup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.errorMsg = msg.Err.Error()
		// Go back to release group results if direct fetch failed
		if m.selectedReleaseGroup == nil {
			m.state = StateReleaseGroupResults
		} else {
			m.state = StateReleaseResults
		}
		return m, nil
	}

	m.releaseDetails = msg.Release

	// If we got here via direct MB ID lookup, we need to create a placeholder release group
	if m.selectedReleaseGroup == nil {
		m.selectedReleaseGroup = &musicbrainz.ReleaseGroup{
			ID:           "", // Unknown
			Title:        m.releaseDetails.Title,
			PrimaryType:  m.releaseDetails.ReleaseType,
			FirstRelease: m.releaseDetails.Date,
			Artist:       m.releaseDetails.Artist,
		}
	}

	// Build tag diffs
	m.buildTagDiffs()
	m.state = StateTagPreview
	m.statusMsg = ""
	m.errorMsg = ""

	return m, nil
}

// handleFileRetagged handles a single file retag completion.
func (m *Model) handleFileRetagged(msg FileRetaggedMsg) (uipopup.Popup, tea.Cmd) {
	if msg.Index < 0 || msg.Index >= len(m.retagStatus) {
		return m, nil
	}

	if msg.Err != nil {
		m.retagStatus[msg.Index].Status = StatusFailed
		m.retagStatus[msg.Index].Error = msg.Err.Error()
		m.failedFiles = append(m.failedFiles, FailedFile{
			Filename: filepath.Base(m.trackPaths[msg.Index]),
			Error:    msg.Err.Error(),
		})
	} else {
		m.retagStatus[msg.Index].Status = StatusComplete
		m.successCount++
	}

	// Start next file or finish
	m.currentFile++
	if m.currentFile < len(m.trackPaths) {
		cmd := m.retagNextFile()
		return m, cmd
	}

	// All files processed, update library with retagged files only
	m.statusMsg = "Updating library..."
	return m, UpdateTracksCmd(m.lib, m.trackPaths)
}

// handleLibraryUpdated handles library update completion after retagging.
func (m *Model) handleLibraryUpdated(msg LibraryUpdatedMsg) (uipopup.Popup, tea.Cmd) {
	m.statusMsg = ""

	if msg.Err != nil {
		// Show error and stay on complete screen
		m.state = StateComplete
		m.errorMsg = "Library update failed: " + msg.Err.Error()
		return m, nil
	}

	// No errors - auto-close and notify app to refresh views
	if len(m.failedFiles) == 0 {
		return m, func() tea.Msg {
			return ActionMsg(Complete{
				AlbumArtist:  m.albumArtist,
				AlbumName:    m.albumName,
				SuccessCount: m.successCount,
				FailedCount:  0,
			})
		}
	}

	// Some files failed - show complete screen
	m.state = StateComplete
	return m, nil
}

// startRetag begins the retagging process.
func (m *Model) startRetag() tea.Cmd {
	m.currentFile = 0
	m.successCount = 0
	m.failedFiles = nil

	// Reset all statuses
	for i := range m.retagStatus {
		m.retagStatus[i].Status = StatusPending
		m.retagStatus[i].Error = ""
	}

	return m.retagNextFile()
}

// retagNextFile retags the next file in the queue.
func (m *Model) retagNextFile() tea.Cmd {
	if m.currentFile >= len(m.trackPaths) {
		return nil
	}

	m.retagStatus[m.currentFile].Status = StatusRetagging

	// Find the matching track in release details
	trackIndex := m.findMatchingTrack(m.currentFile)
	if trackIndex < 0 {
		// No matching track found, use index directly
		trackIndex = m.currentFile
		if trackIndex >= len(m.releaseDetails.Tracks) {
			trackIndex = len(m.releaseDetails.Tracks) - 1
		}
	}

	// Determine disc number
	discNumber := 1
	totalDiscs := m.releaseDetails.DiscCount
	if totalDiscs == 0 {
		totalDiscs = 1
	}
	if trackIndex < len(m.releaseDetails.Tracks) {
		discNumber = m.releaseDetails.Tracks[trackIndex].DiscNumber
		if discNumber == 0 {
			discNumber = 1
		}
	}

	// Get existing genre for preservation if new genre is empty
	existingGenre := ""
	if m.currentFile < len(m.currentTags) {
		existingGenre = m.currentTags[m.currentFile].Genre
	}

	return FileCmd(FileParams{
		Index:         m.currentFile,
		Path:          m.trackPaths[m.currentFile],
		ReleaseGroup:  m.selectedReleaseGroup,
		Release:       m.releaseDetails,
		TrackIndex:    trackIndex,
		DiscNumber:    discNumber,
		TotalDiscs:    totalDiscs,
		ExistingGenre: existingGenre,
	})
}

// findMatchingTrack finds the MusicBrainz track index that best matches the current file.
func (m *Model) findMatchingTrack(fileIndex int) int {
	if fileIndex >= len(m.currentTags) || m.releaseDetails == nil {
		return -1
	}

	currentTag := m.currentTags[fileIndex]

	// No track number - fallback to index
	if currentTag.Track <= 0 {
		return fileIndex
	}

	// Try to match by track number and disc number
	idx := m.findTrackByNumberAndDisc(currentTag.Track, currentTag.Disc)
	if idx >= 0 {
		return idx
	}

	// Fallback: match by position only (ignore disc)
	for i, t := range m.releaseDetails.Tracks {
		if t.Position == currentTag.Track {
			return i
		}
	}

	// Fallback to index
	return fileIndex
}

// findTrackByNumberAndDisc finds a track matching both position and disc number.
func (m *Model) findTrackByNumberAndDisc(trackNum, discNum int) int {
	for i, t := range m.releaseDetails.Tracks {
		if t.Position == trackNum && (discNum == 0 || t.DiscNumber == discNum) {
			return i
		}
	}
	return -1
}

// buildTagDiffs computes the differences between current tags and new MusicBrainz data.
func (m *Model) buildTagDiffs() {
	m.tagDiffs = nil

	if m.releaseDetails == nil || len(m.currentTags) == 0 {
		return
	}

	// Collect unique values from current tags
	collectUnique := func(extract func(t *player.TrackInfo) string) string {
		values := make(map[string]bool)
		for i := range m.currentTags {
			v := extract(&m.currentTags[i])
			if v != "" {
				values[v] = true
			}
		}
		if len(values) == 0 {
			return emptyPlaceholder
		}
		if len(values) == 1 {
			for v := range values {
				return v
			}
		}
		return "(multiple)"
	}

	addDiff := func(field, oldVal, newVal string) {
		if oldVal == "" {
			oldVal = emptyPlaceholder
		}
		if newVal == "" {
			newVal = emptyPlaceholder
		}
		m.tagDiffs = append(m.tagDiffs, TagDiff{
			Field:    field,
			OldValue: oldVal,
			NewValue: newVal,
			Changed:  oldVal != newVal,
		})
	}

	// Build release type string (same as in commands.go)
	releaseType := strings.ToLower(m.selectedReleaseGroup.PrimaryType)
	if len(m.selectedReleaseGroup.SecondaryTypes) > 0 {
		releaseType += "; " + strings.Join(m.selectedReleaseGroup.SecondaryTypes, "; ")
	}

	// Extract original year from original date
	newOriginalYear := ""
	if len(m.selectedReleaseGroup.FirstRelease) >= 4 {
		newOriginalYear = m.selectedReleaseGroup.FirstRelease[:4]
	}

	// Basic tags (matching import popup order)
	addDiff("Artist", "(per track)", "(from MusicBrainz)")
	addDiff("Album Artist", collectUnique(func(t *player.TrackInfo) string { return t.AlbumArtist }), m.releaseDetails.Artist)
	addDiff("Album", collectUnique(func(t *player.TrackInfo) string { return t.Album }), m.releaseDetails.Title)
	addDiff("Track Titles", "(see files)", "(from MusicBrainz)")

	// Date tags
	addDiff("Date", collectUnique(func(t *player.TrackInfo) string { return t.Date }), m.releaseDetails.Date)
	addDiff("Original Date", collectUnique(func(t *player.TrackInfo) string { return t.OriginalDate }), m.selectedReleaseGroup.FirstRelease)
	addDiff("Original Year", collectUnique(func(t *player.TrackInfo) string { return t.OriginalYear }), newOriginalYear)

	// Genre - preserve existing if new is empty
	existingGenre := collectUnique(func(t *player.TrackInfo) string { return t.Genre })
	newGenre := importer.BuildGenreString(m.releaseDetails.Genres, m.selectedReleaseGroup.Genres)
	if newGenre == "" && existingGenre != "" && existingGenre != emptyPlaceholder {
		newGenre = existingGenre + " (preserved)"
	}
	addDiff("Genre", existingGenre, newGenre)

	// Release info
	addDiff("Label", collectUnique(func(t *player.TrackInfo) string { return t.Label }), m.releaseDetails.Label)
	addDiff("Catalog #", collectUnique(func(t *player.TrackInfo) string { return t.CatalogNumber }), m.releaseDetails.CatalogNumber)
	addDiff("Barcode", collectUnique(func(t *player.TrackInfo) string { return t.Barcode }), m.releaseDetails.Barcode)
	addDiff("Media", collectUnique(func(t *player.TrackInfo) string { return t.Media }), m.releaseDetails.Formats)
	addDiff("Release Type", collectUnique(func(t *player.TrackInfo) string { return t.ReleaseType }), releaseType)
	addDiff("Status", collectUnique(func(t *player.TrackInfo) string { return t.ReleaseStatus }), m.releaseDetails.Status)
	addDiff("Country", collectUnique(func(t *player.TrackInfo) string { return t.Country }), m.releaseDetails.Country)
	addDiff("Script", collectUnique(func(t *player.TrackInfo) string { return t.Script }), m.releaseDetails.Script)

	// MusicBrainz IDs
	addDiff("MB Artist ID", collectUnique(func(t *player.TrackInfo) string { return t.MBArtistID }), m.releaseDetails.ArtistID)
	addDiff("MB Release ID", collectUnique(func(t *player.TrackInfo) string { return t.MBReleaseID }), m.releaseDetails.ID)
}

// filterAlbumReleaseGroups filters release groups to albums only.
func filterAlbumReleaseGroups(groups []musicbrainz.ReleaseGroup) []musicbrainz.ReleaseGroup {
	var filtered []musicbrainz.ReleaseGroup
	for i := range groups {
		g := &groups[i]
		// Include albums, exclude live and compilations
		if g.PrimaryType == "Album" {
			hasExcluded := false
			for _, st := range g.SecondaryTypes {
				if st == "Live" || st == "Compilation" {
					hasExcluded = true
					break
				}
			}
			if !hasExcluded {
				filtered = append(filtered, groups[i])
			}
		}
	}
	// If no albums found, return all results
	if len(filtered) == 0 {
		return groups
	}
	return filtered
}

// sortReleasesByDate sorts releases by date ascending (oldest first).
func sortReleasesByDate(releases []musicbrainz.Release) []musicbrainz.Release {
	sorted := make([]musicbrainz.Release, len(releases))
	copy(sorted, releases)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date < sorted[j].Date
	})
	return sorted
}
