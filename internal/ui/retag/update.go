package retag

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
	uipopup "github.com/llehouerou/waves/internal/ui/popup"
)

const (
	emptyPlaceholder   = "(empty)"
	msgLoadingReleases = "Loading releases..."
)

// Compile-time check that Model implements popup.Popup.
var _ uipopup.Popup = (*Model)(nil)

// Init asks the core for the tags of the album's files.
func (m *Model) Init() tea.Cmd {
	return readAlbumTagsCmd(m.lib, m.albumArtist, m.albumName)
}

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (uipopup.Popup, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case TagsReadMsg:
		return m.handleTagsRead(msg)
	case mbworkflow.SearchResultMsg:
		return m.handleReleaseGroupSearchResult(msg)
	case mbworkflow.ReleasesResultMsg:
		return m.handleReleasesFetched(msg)
	case mbworkflow.ReleaseDetailsResultMsg:
		return m.handleReleaseDetailsFetched(msg)
	case mbworkflow.CoverArtResultMsg:
		return m.handleCoverArtFetched(msg)

	// The retag runs in the core: the app hands over its events. Those of a
	// retag this popup did not start are not its own.
	case core.RetagProgressed:
		if m.state == StateRetagging && m.Retags(msg.AlbumArtist, msg.Album) {
			m.files = msg.Files
		}
	case core.RetagFinished:
		if m.state == StateRetagging && m.Retags(msg.AlbumArtist, msg.Album) {
			return m.handleRetagFinished(msg)
		}
	}
	return m, nil
}

// complete tells the app the popup is done with the retag it followed.
func (m *Model) complete() tea.Msg {
	return ActionMsg(Complete{AlbumArtist: m.albumArtist, AlbumName: m.albumName})
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
		return m, mbworkflow.SearchCmd(m.mbClient, query)
	default:
		// Update text input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}
}

// handleEscape handles the escape key.
func (m *Model) handleEscape() (uipopup.Popup, tea.Cmd) {
	if m.state == StateComplete {
		return m, m.complete
	}
	// While retagging too: the core finishes the retag without the popup.
	return m, func() tea.Msg { return ActionMsg(Close{}) }
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
		m.statusMsg = msgLoadingReleases
		return m, mbworkflow.FetchReleasesCmd(m.mbClient, m.selectedReleaseGroup.ID)

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
		return m, mbworkflow.FetchReleaseDetailsCmd(m.mbClient, release.ID)

	case StateTagPreview:
		// Wait for cover art to be fetched before starting retag
		if m.coverArtFetched {
			m.startRetag()
		}

	case StateComplete:
		return m, m.complete
	}
	return m, nil
}

// startRetag asks the core to retag the album as the release shown. The core
// returns at once: its events tell the rest.
func (m *Model) startRetag() {
	err := m.lib.Retag(core.RetagRequest{
		AlbumArtist:  m.albumArtist,
		Album:        m.albumName,
		Release:      m.releaseDetails,
		ReleaseGroup: m.selectedReleaseGroup,
		Cover:        m.coverArt,
	})
	if err != nil {
		m.errorMsg = err.Error()
		return
	}
	m.errorMsg = ""
	m.files = make([]core.FileProgress, len(m.currentTags))
	for i := range m.currentTags {
		m.files[i] = core.FileProgress{Path: m.currentTags[i].Path}
	}
	m.state = StateRetagging
}

// handleRetagFinished takes the end of the retag: with nothing to tell, the
// popup is done; a failure stays on screen.
func (m *Model) handleRetagFinished(msg core.RetagFinished) (uipopup.Popup, tea.Cmd) {
	m.files = msg.Files
	if msg.Err == nil && core.AllDone(msg.Files) {
		return m, m.complete
	}
	if msg.Err != nil {
		m.errorMsg = "Library update failed: " + msg.Err.Error()
	}
	m.state = StateComplete
	return m, nil
}

// handleDown handles down/j key.
func (m *Model) handleDown() (uipopup.Popup, tea.Cmd) {
	maxVisible := max(m.Height()-12, 5)
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
	maxVisible := max(m.Height()-12, 5)
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
		m.errorMsg = "" // a retag the core refused is not about the releases
		// If releases list is empty (came via direct ID lookup), fetch releases for the release group
		if len(m.releases) == 0 && m.releaseDetails != nil && m.releaseDetails.ReleaseGroupID != "" {
			releaseGroupID := m.releaseDetails.ReleaseGroupID
			// Update the selected release group ID so we can go back further if needed
			if m.selectedReleaseGroup != nil {
				m.selectedReleaseGroup.ID = releaseGroupID
			}
			m.state = StateReleaseLoading
			m.statusMsg = msgLoadingReleases
			m.releaseDetails = nil
			m.tagDiffs = nil
			m.skipAutoSelect = true // User navigated back, don't auto-select
			return m, mbworkflow.FetchReleasesCmd(m.mbClient, releaseGroupID)
		}
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

// handleTagsRead takes the tags of the album's files and looks the album up
// on MusicBrainz, from the most precise ID they carry.
func (m *Model) handleTagsRead(msg TagsReadMsg) (uipopup.Popup, tea.Cmd) {
	if msg.Err != nil {
		// No file to retag: nothing to search a release for.
		m.errorMsg = msg.Err.Error()
		m.state = StateReleaseGroupResults
		return m, nil
	}
	m.currentTags = msg.Tags

	// The first file that carries an ID speaks for the album
	for i := range m.currentTags {
		tag := &m.currentTags[i]
		if m.foundMBReleaseID == "" {
			m.foundMBReleaseID = tag.MBReleaseID
		}
		if m.foundMBReleaseGroupID == "" {
			m.foundMBReleaseGroupID = tag.MBReleaseGroupID
		}
		if m.foundMBArtistID == "" {
			m.foundMBArtistID = tag.MBArtistID
		}
	}

	m.state = StateSearching

	// Priority 1: If we found a MusicBrainz Release ID, fetch that release directly
	if m.foundMBReleaseID != "" {
		m.searchMethod = "Found release ID in tags"
		m.statusMsg = "Loading release from MusicBrainz..."
		return m, FetchReleaseByIDCmd(m.mbClient, m.foundMBReleaseID)
	}

	// Priority 2: If we found a Release Group ID, fetch releases for that group
	if m.foundMBReleaseGroupID != "" {
		m.searchMethod = "Found release group ID in tags"
		m.statusMsg = "Loading releases for release group..."
		return m, mbworkflow.FetchReleasesCmd(m.mbClient, m.foundMBReleaseGroupID)
	}

	// Priority 3: If we found an Artist ID, browse their release groups
	if m.foundMBArtistID != "" {
		m.searchMethod = "Browsing artist's releases (artist ID in tags)"
		m.statusMsg = "Loading artist's release groups..."
		return m, mbworkflow.FetchArtistReleaseGroupsCmd(m.mbClient, m.foundMBArtistID)
	}

	// Priority 4: Search by artist name + album name
	m.searchMethod = "Searching by artist and album name"
	m.statusMsg = "Searching MusicBrainz..."
	return m, mbworkflow.SearchCmd(m.mbClient, m.albumArtist+" "+m.albumName)
}

// handleReleaseGroupSearchResult handles release group search results.
func (m *Model) handleReleaseGroupSearchResult(msg mbworkflow.SearchResultMsg) (uipopup.Popup, tea.Cmd) {
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
		m.statusMsg = msgLoadingReleases
		return m, mbworkflow.FetchReleasesCmd(m.mbClient, m.selectedReleaseGroup.ID)
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
func (m *Model) releaseGroupMatchScore(rg *core.MBReleaseGroup) int {
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
func (m *Model) handleReleasesFetched(msg mbworkflow.ReleasesResultMsg) (uipopup.Popup, tea.Cmd) {
	if msg.Err != nil {
		m.errorMsg = msg.Err.Error()
		m.state = StateReleaseResults
		m.skipAutoSelect = false
		return m, nil
	}

	m.releases = sortReleasesByDate(msg.Releases)

	// Sort releases with matching track count first
	m.sortReleasesByTrackCountMatch()

	// Find releases with matching track count - auto-select only if not navigating back
	if !m.skipAutoSelect {
		matchingIndices := m.findReleasesWithMatchingTrackCount()
		if len(matchingIndices) == 1 {
			// Auto-select the only release with matching track count
			release := m.releases[matchingIndices[0]]
			m.state = StateReleaseDetailsLoading
			m.statusMsg = "Loading release details..."
			return m, mbworkflow.FetchReleaseDetailsCmd(m.mbClient, release.ID)
		}
	}

	m.releaseCursor.Reset()
	m.releaseCursor.ClampToBounds(len(m.releases))
	m.state = StateReleaseResults
	m.statusMsg = ""
	m.errorMsg = ""
	m.skipAutoSelect = false // Reset for next time

	return m, nil
}

// sortReleasesByTrackCountMatch sorts releases with matching track count first.
func (m *Model) sortReleasesByTrackCountMatch() {
	localTrackCount := len(m.currentTags)
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
	localTrackCount := len(m.currentTags)
	var matches []int
	for i := range m.releases {
		if m.releases[i].TrackCount == localTrackCount {
			matches = append(matches, i)
		}
	}
	return matches
}

// handleReleaseDetailsFetched handles full release details.
func (m *Model) handleReleaseDetailsFetched(msg mbworkflow.ReleaseDetailsResultMsg) (uipopup.Popup, tea.Cmd) {
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

	m.releaseDetails = msg.Details

	// If we got here via direct MB ID lookup, we need to create a placeholder release group
	if m.selectedReleaseGroup == nil {
		m.selectedReleaseGroup = &core.MBReleaseGroup{
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

	// Start fetching cover art in background
	if m.releaseDetails.ID != "" {
		return m, mbworkflow.FetchCoverArtCmd(m.mbClient, m.releaseDetails.ID)
	}

	return m, nil
}

// handleCoverArtFetched handles the cover art fetch result.
func (m *Model) handleCoverArtFetched(msg mbworkflow.CoverArtResultMsg) (uipopup.Popup, tea.Cmd) {
	m.coverArtFetched = true
	if msg.Err == nil {
		m.coverArt = msg.Data // may be nil if not found (404), that's ok
	}
	return m, nil
}

// buildTagDiffs computes the differences between current tags and new MusicBrainz data.
func (m *Model) buildTagDiffs() {
	m.tagDiffs = nil

	if m.releaseDetails == nil || len(m.currentTags) == 0 {
		return
	}

	// Collect unique values from current tags
	collectUnique := func(extract func(t *core.FileTags) string) string {
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
	addDiff("Album Artist", collectUnique(func(t *core.FileTags) string { return t.AlbumArtist }), m.releaseDetails.Artist)
	addDiff("Album", collectUnique(func(t *core.FileTags) string { return t.Album }), m.releaseDetails.Title)
	addDiff("Track Titles", "(see files)", "(from MusicBrainz)")

	// Date tags
	addDiff("Date", collectUnique(func(t *core.FileTags) string { return t.Date }), m.releaseDetails.Date)
	addDiff("Original Date", collectUnique(func(t *core.FileTags) string { return t.OriginalDate }), m.selectedReleaseGroup.FirstRelease)
	addDiff("Original Year", collectUnique(func(t *core.FileTags) string { return t.OriginalYear() }), newOriginalYear)

	// Genre - preserve existing if new is empty
	existingGenre := collectUnique(func(t *core.FileTags) string { return t.Genre })
	newGenre := core.BuildGenreString(m.releaseDetails.Genres, m.selectedReleaseGroup.Genres)
	if newGenre == "" && existingGenre != "" && existingGenre != emptyPlaceholder {
		newGenre = existingGenre + " (preserved)"
	}
	addDiff("Genre", existingGenre, newGenre)

	// Release info
	addDiff("Label", collectUnique(func(t *core.FileTags) string { return t.Label }), m.releaseDetails.Label)
	addDiff("Catalog #", collectUnique(func(t *core.FileTags) string { return t.CatalogNumber }), m.releaseDetails.CatalogNumber)
	addDiff("Barcode", collectUnique(func(t *core.FileTags) string { return t.Barcode }), m.releaseDetails.Barcode)
	addDiff("Media", collectUnique(func(t *core.FileTags) string { return t.Media }), m.releaseDetails.Formats)
	addDiff("Release Type", collectUnique(func(t *core.FileTags) string { return t.ReleaseType }), releaseType)
	addDiff("Status", collectUnique(func(t *core.FileTags) string { return t.ReleaseStatus }), m.releaseDetails.Status)
	addDiff("Country", collectUnique(func(t *core.FileTags) string { return t.Country }), m.releaseDetails.Country)
	addDiff("Script", collectUnique(func(t *core.FileTags) string { return t.Script }), m.releaseDetails.Script)

	// MusicBrainz IDs
	addDiff("MB Artist ID", collectUnique(func(t *core.FileTags) string { return t.MBArtistID }), m.releaseDetails.ArtistID)
	addDiff("MB Release ID", collectUnique(func(t *core.FileTags) string { return t.MBReleaseID }), m.releaseDetails.ID)
}

// filterAlbumReleaseGroups filters release groups to albums only.
func filterAlbumReleaseGroups(groups []core.MBReleaseGroup) []core.MBReleaseGroup {
	var filtered []core.MBReleaseGroup
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
func sortReleasesByDate(releases []core.MBRelease) []core.MBRelease {
	sorted := make([]core.MBRelease, len(releases))
	copy(sorted, releases)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date < sorted[j].Date
	})
	return sorted
}
