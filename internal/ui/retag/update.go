package retag

import (
	"errors"
	"slices"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/ui/cursor"
	"github.com/llehouerou/waves/internal/ui/fit"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
	uipopup "github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/request"
	"github.com/llehouerou/waves/internal/ui/tagdiff"
)

const (
	emptyPlaceholder   = "(empty)"
	msgLoadingReleases = "Loading releases..."
)

// Compile-time check that Model implements popup.Popup.
var _ uipopup.Popup = (*Model)(nil)

// Init asks the core for the tags of the album's files.
func (m *Model) Init() tea.Cmd {
	return m.tagsRead.Ask(readAlbumTags(m.lib, m.albumArtist, m.albumName))
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

// complete tells the app the popup is done with the retag it followed, and
// the album to show.
func (m *Model) complete() tea.Msg {
	return ActionMsg(Complete{AlbumArtist: m.shownArtist, Album: m.shownAlbum})
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
		return m.handleDown(key)
	case "k", "up":
		return m.handleUp(key)
	case "backspace":
		return m.handleBackspace()
	case "/":
		return m.handleSearchToggle()
	case "a":
		m.toggleAlbumsOnly()
	default:
		if fit.Scrolls(key) {
			m.scroll(key)
		}
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
		return m.ask(mbworkflow.SearchReleaseGroups(m.mbClient, query))
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
		return m.listReleasesOf(&m.releaseGroups[idx])

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
		return m.ask(mbworkflow.ReleaseDetails(m.mbClient, release.ID))

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
		ReleaseGroup: m.releaseDetails.ReleaseGroup,
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
// popup is done, and the album is as the library now has it; a failure stays
// on screen, and the album is the one the popup was opened on, whose names
// the files that failed keep.
func (m *Model) handleRetagFinished(msg core.RetagFinished) (uipopup.Popup, tea.Cmd) {
	m.files = msg.Files
	if msg.Err == nil && core.AllDone(msg.Files) {
		m.shownArtist, m.shownAlbum = msg.LibraryAlbumArtist, msg.LibraryAlbum
		return m, m.complete
	}
	m.shownArtist, m.shownAlbum = m.albumArtist, m.albumName
	if msg.Err != nil {
		m.errorMsg = "Library update failed: " + msg.Err.Error()
	}
	m.state = StateComplete
	m.failedOffset = 0
	return m, nil
}

// handleDown handles down/j key: the cursor of a list, the tags of the tag
// preview, the failed files of the complete view.
func (m *Model) handleDown(key string) (uipopup.Popup, tea.Cmd) {
	switch m.state { //nolint:exhaustive // only handle list states
	case StateReleaseGroupResults:
		m.moveCursor(&m.releaseGroupCursor, m.releaseGroupCursor.Pos()+1, len(m.releaseGroups))
	case StateReleaseResults:
		m.moveCursor(&m.releaseCursor, m.releaseCursor.Pos()+1, len(m.releases))
	default:
		m.scroll(key)
	}
	return m, nil
}

// handleUp handles up/k key: the cursor of a list, the tags of the tag
// preview, the failed files of the complete view.
func (m *Model) handleUp(key string) (uipopup.Popup, tea.Cmd) {
	switch m.state { //nolint:exhaustive // only handle list states
	case StateReleaseGroupResults:
		m.moveCursor(&m.releaseGroupCursor, m.releaseGroupCursor.Pos()-1, len(m.releaseGroups))
	case StateReleaseResults:
		m.moveCursor(&m.releaseCursor, m.releaseCursor.Pos()-1, len(m.releases))
	default:
		m.scroll(key)
	}
	return m, nil
}

// moveCursor moves the cursor c of the list the popup shows to row to,
// within its n rows, and its page with it: the page the view shows from the
// one it showed, the cursor in view.
func (m *Model) moveCursor(c *cursor.Cursor, to, n int) {
	if n == 0 {
		return
	}
	c.SetPos(min(max(to, 0), n-1))
	c.SetOffset(m.page().First())
}

// scroll scrolls the list the user scrolls by hand by key, stopping at its
// first item and at its last page: the tags of the tag preview, the failed
// files of the complete view. Elsewhere it does nothing.
func (m *Model) scroll(key string) {
	switch m.state { //nolint:exhaustive // the other lists do not scroll by hand
	case StateTagPreview:
		m.tagOffset = m.page().Scroll(key)
	case StateComplete:
		m.failedOffset = m.page().Scroll(key)
	}
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
		if len(m.releases) == 0 && m.releaseDetails != nil {
			releaseGroupID := m.releaseDetails.ReleaseGroup.ID
			m.state = StateReleaseLoading
			m.statusMsg = msgLoadingReleases
			m.leavePreview()
			m.skipAutoSelect = true // User navigated back, don't auto-select
			return m.ask(mbworkflow.ReleaseGroupReleases(m.mbClient, releaseGroupID))
		}
		m.state = StateReleaseResults
		m.leavePreview()
	}
	return m, nil
}

// leavePreview forgets the release shown in the tag preview, and its cover.
func (m *Model) leavePreview() {
	m.cover.Forget()
	m.releaseDetails = nil
	m.tagDiffs = nil
	m.tagOffset = 0
	m.coverArt = nil
	m.coverArtFetched = false
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
// on MusicBrainz, from the most precise ID they carry. It takes them once,
// while it waits for them: those read for a popup closed since, and its own
// come again, are dropped.
func (m *Model) handleTagsRead(msg TagsReadMsg) (uipopup.Popup, tea.Cmd) {
	if !m.tagsRead.Take(msg.Request) {
		return m, nil
	}
	if msg.Err != nil {
		// No file to retag: nothing to search a release for.
		m.errorMsg = msg.Err.Error()
		m.state = StateReleaseGroupResults
		return m, nil
	}
	m.currentTags = msg.Tags

	// The first file that carries an ID speaks for the album. MusicBrainz is
	// asked with one ID: the first of a list.
	for i := range m.currentTags {
		tag := &m.currentTags[i]
		if m.foundMBReleaseID == "" {
			m.foundMBReleaseID = tags.FirstValue(tag.MBReleaseID)
		}
		if m.foundMBReleaseGroupID == "" {
			m.foundMBReleaseGroupID = tags.FirstValue(tag.MBReleaseGroupID)
		}
		if m.foundMBArtistID == "" {
			m.foundMBArtistID = tags.FirstValue(tag.MBArtistID)
		}
	}

	m.state = StateSearching

	// Priority 1: If we found a MusicBrainz Release ID, fetch that release directly
	if m.foundMBReleaseID != "" {
		m.searchMethod = "Found release ID in tags"
		m.statusMsg = "Loading release from MusicBrainz..."
		return m.ask(mbworkflow.ReleaseDetails(m.mbClient, m.foundMBReleaseID))
	}

	// Priority 2: If we found a Release Group ID, fetch releases for that group
	if m.foundMBReleaseGroupID != "" {
		m.searchMethod = "Found release group ID in tags"
		m.statusMsg = "Loading releases for release group..."
		return m.ask(mbworkflow.ReleaseGroupReleases(m.mbClient, m.foundMBReleaseGroupID))
	}

	// Priority 3: If we found an Artist ID, browse their release groups
	if m.foundMBArtistID != "" {
		m.searchMethod = "Browsing artist's releases (artist ID in tags)"
		m.statusMsg = "Loading artist's release groups..."
		return m.ask(mbworkflow.ArtistReleaseGroups(m.mbClient, m.foundMBArtistID))
	}

	// Priority 4: Search by artist name + album name
	m.searchMethod = "Searching by artist and album name"
	m.statusMsg = "Searching MusicBrainz..."
	return m.ask(mbworkflow.SearchReleaseGroups(m.mbClient, m.albumArtist+" "+m.albumName))
}

// ask sends a MusicBrainz request. From then on, the popup takes the result
// of that request only.
func (m *Model) ask(build func(request.ID) tea.Cmd) (uipopup.Popup, tea.Cmd) {
	return m, m.waiting.Ask(build)
}

// handleReleaseGroupSearchResult handles release group search results. The
// popup drops a result it is not waiting for: it answers a request made
// before, or by a popup closed since.
func (m *Model) handleReleaseGroupSearchResult(msg mbworkflow.SearchResultMsg) (uipopup.Popup, tea.Cmd) {
	if !m.waiting.Take(msg.Request) {
		return m, nil
	}
	if msg.Err != nil {
		m.errorMsg = msg.Err.Error()
		m.state = StateReleaseGroupResults
		return m, nil
	}

	// Sort results: exact matches first, then by artist match, then album match
	m.foundGroups = m.sortByRelevance(msg.ReleaseGroups)
	m.listReleaseGroups()

	if match := m.selectedMatch(); match >= 0 {
		return m.listReleasesOf(&m.releaseGroups[match])
	}

	m.releaseGroupCursor.Reset()
	m.releaseGroupCursor.ClampToBounds(len(m.releaseGroups))
	m.state = StateReleaseGroupResults
	m.statusMsg = ""
	m.errorMsg = ""

	return m, nil
}

// listReleasesOf loads the releases of a release group chosen from the list,
// by the user or automatically.
// The popup keeps its title only, for the header: a group from a list (a
// search has no genres, its index may lag) never tags the album.
func (m *Model) listReleasesOf(group *core.MBReleaseGroup) (uipopup.Popup, tea.Cmd) {
	m.listedGroupTitle = group.Title
	m.state = StateReleaseLoading
	m.statusMsg = msgLoadingReleases
	return m.ask(mbworkflow.ReleaseGroupReleases(m.mbClient, group.ID))
}

// toggleAlbumsOnly turns the albums-only filter of the list of release groups
// off and on. It only changes what is listed: the cursor stays on a listed row.
func (m *Model) toggleAlbumsOnly() {
	if m.state != StateReleaseGroupResults {
		return
	}
	m.albumsOnly = !m.albumsOnly
	m.listReleaseGroups()
	m.releaseGroupCursor.ClampToBounds(len(m.releaseGroups))
	m.releaseGroupCursor.SetOffset(m.page().First())
}

// listReleaseGroups lists the groups found that the filter keeps: every one
// with the filter off, the studio albums only with it on.
func (m *Model) listReleaseGroups() {
	if !m.albumsOnly {
		m.releaseGroups = m.foundGroups
		return
	}
	m.releaseGroups = nil
	for i := range m.foundGroups {
		if m.foundGroups[i].IsStudioAlbum() {
			m.releaseGroups = append(m.releaseGroups, m.foundGroups[i])
		}
	}
}

// selectedMatch returns the index of the listed group the popup selects by
// itself, -1 for none: the one group with the album's artist and title, or,
// among several, the one studio album.
func (m *Model) selectedMatch() int {
	matches := m.findExactMatches()
	if len(matches) == 1 {
		return matches[0]
	}
	selected := -1
	for _, i := range matches {
		if m.releaseGroups[i].IsStudioAlbum() {
			if selected >= 0 {
				return -1
			}
			selected = i
		}
	}
	return selected
}

// sortByRelevance returns groups with best matches first.
func (m *Model) sortByRelevance(groups []core.MBReleaseGroup) []core.MBReleaseGroup {
	sorted := slices.Clone(groups)
	sort.SliceStable(sorted, func(i, j int) bool {
		return m.releaseGroupMatchScore(&sorted[i]) > m.releaseGroupMatchScore(&sorted[j])
	})
	return sorted
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

// handleReleasesFetched handles releases fetched for a release group, if the
// popup waits for them.
func (m *Model) handleReleasesFetched(msg mbworkflow.ReleasesResultMsg) (uipopup.Popup, tea.Cmd) {
	if !m.waiting.Take(msg.Request) {
		return m, nil
	}
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
			return m.ask(mbworkflow.ReleaseDetails(m.mbClient, release.ID))
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

// handleReleaseDetailsFetched handles full release details, if the popup
// waits for them.
func (m *Model) handleReleaseDetailsFetched(msg mbworkflow.ReleaseDetailsResultMsg) (uipopup.Popup, tea.Cmd) {
	if !m.waiting.Take(msg.Request) {
		return m, nil
	}
	err := msg.Err
	if err == nil && msg.Details.ReleaseGroup == nil {
		// Every MusicBrainz release has one: without it, nothing to tag.
		err = errors.New("MusicBrainz gave the release no release group")
	}
	if err != nil {
		m.errorMsg = err.Error()
		// Back to the releases it was picked from, if any
		if len(m.releases) == 0 {
			m.state = StateReleaseGroupResults
		} else {
			m.state = StateReleaseResults
		}
		return m, nil
	}

	m.releaseDetails = msg.Details // no cover yet: leavePreview forgot the one before

	// Build tag diffs
	m.buildTagDiffs()
	m.tagOffset = 0
	m.state = StateTagPreview
	m.statusMsg = ""
	m.errorMsg = ""

	// Start fetching cover art in background
	if m.releaseDetails.ID != "" {
		return m, m.cover.Ask(mbworkflow.CoverArt(m.mbClient, m.releaseDetails.ID))
	}

	return m, nil
}

// handleCoverArtFetched takes the cover of the release shown in the tag
// preview. The cover of a release shown before, or fetched by a popup
// closed since, is not its own.
func (m *Model) handleCoverArtFetched(msg mbworkflow.CoverArtResultMsg) (uipopup.Popup, tea.Cmd) {
	if !m.cover.Take(msg.Request) {
		return m, nil
	}
	m.coverArtFetched = true
	if msg.Err == nil {
		m.coverArt = msg.Data // may be nil if not found (404), that's ok
	}
	return m, nil
}

// buildTagDiffs compares the tags of the files the retag writes, the ones the
// release has a track for, with what it writes on each of them.
func (m *Model) buildTagDiffs() {
	m.tagDiffs = nil

	if m.releaseDetails == nil || len(m.currentTags) == 0 {
		return
	}
	release := m.releaseDetails
	group := release.ReleaseGroup // the group of the release shown, never the one chosen from a list

	var files []tagdiff.File
	for i := range m.currentTags {
		if track := core.RetagTrack(&m.currentTags[i], i, release); track >= 0 {
			files = append(files, tagdiff.File{Tags: &m.currentTags[i], Track: &release.Tracks[track]})
		}
	}

	add := func(field string, line tagdiff.Line, newValue string) {
		m.tagDiffs = append(m.tagDiffs, TagDiff{Field: field, OldValue: show(line.Held), NewValue: newValue, Changed: line.Changed})
	}
	// byTrack compares a tag the retag writes from each file's track.
	byTrack := func(held func(*core.FileTags) string, written func(*core.MBTrack) string) tagdiff.Line {
		return tagdiff.Compare(files, held, tagdiff.Track(written))
	}
	// byRelease adds the line of a tag the retag writes value on every file.
	byRelease := func(field string, held func(*core.FileTags) string, value string) {
		newValue := value
		if newValue == "" {
			newValue = emptyPlaceholder
		}
		add(field, tagdiff.Compare(files, held, tagdiff.Every(value)), newValue)
	}

	newOriginalYear := group.FirstRelease
	if len(newOriginalYear) >= 4 {
		newOriginalYear = newOriginalYear[:4]
	}

	// Basic tags (matching import popup order)
	artists := byTrack(func(t *core.FileTags) string { return t.Artist },
		func(track *core.MBTrack) string { return core.TrackArtist(release, track) })
	add("Artist", artists, show(artists.Written))
	byRelease("Album Artist", func(t *core.FileTags) string { return t.AlbumArtist }, release.Artist)
	byRelease("Album", func(t *core.FileTags) string { return t.Album }, release.Title)
	titles := byTrack(func(t *core.FileTags) string { return t.Title },
		func(track *core.MBTrack) string { return track.Title })
	m.tagDiffs = append(m.tagDiffs, TagDiff{Field: "Track Titles", OldValue: "(see files)", NewValue: "(from MusicBrainz)", Changed: titles.Changed})

	// Date tags
	byRelease("Date", func(t *core.FileTags) string { return t.Date }, release.Date)
	byRelease("Original Date", func(t *core.FileTags) string { return t.OriginalDate }, group.FirstRelease)
	byRelease("Original Year", func(t *core.FileTags) string { return t.OriginalYear() }, newOriginalYear)

	// Genre: with none on the release, each file keeps its own
	genre := func(t *core.FileTags) string { return t.Genre }
	if newGenre := core.BuildGenreString(release.Genres, group.Genres); newGenre != "" {
		byRelease("Genre", genre, newGenre)
	} else {
		kept := tagdiff.Kept(files, genre)
		newValue := emptyPlaceholder
		if held := show(kept.Held); held != emptyPlaceholder {
			newValue = held + " (preserved)"
		}
		add("Genre", kept, newValue)
	}

	// Release info
	byRelease("Label", func(t *core.FileTags) string { return t.Label }, release.Label)
	byRelease("Catalog #", func(t *core.FileTags) string { return t.CatalogNumber }, release.CatalogNumber)
	byRelease("Barcode", func(t *core.FileTags) string { return t.Barcode }, release.Barcode)
	medias := byTrack(func(t *core.FileTags) string { return t.Media },
		func(track *core.MBTrack) string { return track.MediumFormat })
	add("Media", medias, show(medias.Written))
	byRelease("Release Type", func(t *core.FileTags) string { return t.ReleaseType }, core.BuildReleaseType(group))
	byRelease("Status", func(t *core.FileTags) string { return t.ReleaseStatus }, release.Status)
	byRelease("Country", func(t *core.FileTags) string { return t.Country }, release.Country)
	byRelease("Script", func(t *core.FileTags) string { return t.Script }, release.Script)

	// MusicBrainz IDs
	artistIDs := byTrack(func(t *core.FileTags) string { return t.MBArtistID },
		func(track *core.MBTrack) string { return core.TrackArtistID(release, track) })
	add("MB Artist ID", artistIDs, show(artistIDs.Written))
	byRelease("MB Release ID", func(t *core.FileTags) string { return t.MBReleaseID }, release.ID)
	byRelease("MB Group ID", func(t *core.FileTags) string { return t.MBReleaseGroupID }, group.ID)
}

// show is how the preview shows the values of a tag on files: emptyPlaceholder
// when none has one, the value when they all hold the same, "(multiple)"
// otherwise.
func show(values tagdiff.Values) string {
	switch {
	case values.Several():
		return "(multiple)"
	case values.Value == "":
		return emptyPlaceholder
	default:
		return values.Value
	}
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
