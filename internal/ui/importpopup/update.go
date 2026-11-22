package importpopup

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/fit"
	uipopup "github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/tagdiff"
)

// Compile-time check that Model implements popup.Popup.
var _ uipopup.Popup = (*Model)(nil)

const emptyValue = "(empty)"

// Init asks the core for the plan of the import.
func (m *Model) Init() tea.Cmd {
	return m.prepared.Ask(prepare(m.importer, m.download.ID))
}

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (uipopup.Popup, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case PreparedMsg:
		// The plan asked by Init, once: the one of a popup closed since is
		// dropped, of the same download or not.
		if m.prepared.Take(msg.Request) {
			m.handlePrepared(msg)
		}
	// The import runs in the core: the app hands over its events. Those of an
	// import this popup did not start, or no longer follows, are not its own.
	case core.ImportProgressed:
		if msg.DownloadID == m.download.ID && m.state == StateImporting {
			m.files = msg.Files
		}
	case core.ImportFinished:
		if msg.DownloadID == m.download.ID && m.state == StateImporting {
			m.files = msg.Files
			m.err = ""
			if msg.Err != nil {
				m.err = "Library not updated: " + msg.Err.Error()
			}
			m.state = StateComplete
			m.failedOffset = 0
		}
	}
	return m, nil
}

// handleKey handles key presses based on current state.
func (m *Model) handleKey(msg tea.KeyMsg) (uipopup.Popup, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		return m.handleEscape()
	case "enter":
		return m.handleEnter()
	case "j", "down":
		m.selectSourceOrScroll(1, key)
	case "k", "up":
		m.selectSourceOrScroll(-1, key)
	default:
		if fit.Scrolls(key) {
			m.scroll(key)
		}
	}

	return m, nil
}

func closeCmd() tea.Msg { return ActionMsg(Close{}) }

// handleEscape handles the escape key - goes back or closes.
func (m *Model) handleEscape() (uipopup.Popup, tea.Cmd) {
	if m.state == StatePathPreview {
		m.state = StateTagPreview
		m.err = ""
		return m, nil
	}
	// While importing too: the core finishes the import without the popup.
	return m, closeCmd
}

// handleEnter handles the enter key - proceeds to next step.
func (m *Model) handleEnter() (uipopup.Popup, tea.Cmd) {
	switch m.state {
	case StateTagPreview:
		if m.planned() {
			m.state = StatePathPreview
		}
	case StatePathPreview:
		m.startImport()
	case StateImporting:
		// Can't interact during import
	case StateComplete:
		return m, closeCmd
	}
	return m, nil
}

// planned reports whether the core has planned the import.
func (m *Model) planned() bool {
	return m.plan.Release != nil
}

// selectSourceOrScroll moves the library source cursor of the path preview
// by delta when there are several sources to choose from, and scrolls the
// list of the view by key otherwise.
func (m *Model) selectSourceOrScroll(delta int, key string) {
	if n := len(m.librarySources); m.state == StatePathPreview && n > 1 {
		m.selectedSource = (m.selectedSource + delta + n) % n
		return
	}
	m.scroll(key)
}

// scroll moves the list of the view the popup shows by key, stopping at its
// first item and at its last page. The progress list follows the import: it
// does not scroll by hand.
func (m *Model) scroll(key string) {
	var offset *int
	switch m.state {
	case StateTagPreview:
		offset = &m.tagOffset
	case StatePathPreview:
		offset = &m.pathOffset
	case StateComplete:
		offset = &m.failedOffset
	case StateImporting:
		return
	}
	*offset = m.page().Scroll(key)
}

// handlePrepared takes the plan of the import, or why there is none.
func (m *Model) handlePrepared(msg PreparedMsg) {
	m.preparing = false
	if msg.Err != nil {
		m.err = msg.Err.Error()
		return
	}
	m.err = ""
	m.plan = msg.Plan
	m.tagOffset, m.pathOffset = 0, 0
	m.buildTagDiffs()
}

// startImport asks the core to import the plan into the selected library
// source. The core returns at once: its events tell the rest.
func (m *Model) startImport() {
	if len(m.librarySources) == 0 {
		return // the path preview says how to add one and offers no Enter
	}
	if err := m.importer.Import(m.plan, m.librarySources[m.selectedSource]); err != nil {
		m.err = err.Error()
		return
	}
	m.err = ""
	m.files = make([]core.FileProgress, len(m.plan.Files))
	for i := range m.plan.Files {
		m.files[i] = core.FileProgress{Path: m.plan.Files[i].Source}
	}
	m.state = StateImporting
}

// buildTagDiffs compares the tags of the files the import writes, the files
// of the plan that have a track, with what it writes on each of them.
func (m *Model) buildTagDiffs() {
	release := m.plan.Release
	releaseGroup := m.plan.ReleaseGroup

	var files []tagdiff.File
	for i := range m.plan.Files {
		if track := m.plan.Files[i].Track; track >= 0 {
			files = append(files, tagdiff.File{Tags: &m.plan.Files[i].Tags, Track: &release.Tracks[track]})
		}
	}
	byTrack := func(held func(*core.FileTags) string, written func(*core.MBTrack) string) tagdiff.Line {
		return tagdiff.Compare(files, held, tagdiff.Track(written))
	}
	byRelease := func(field string, held func(*core.FileTags) string, value string) TagDiff {
		line := tagdiff.Compare(files, held, tagdiff.Every(value))
		return TagDiff{Field: field, OldValue: show(line.Held), NewValue: value, Changed: line.Changed}
	}

	newOriginalDate, newOriginalYear := "", ""
	var releaseGroupGenres []string
	if releaseGroup != nil {
		newOriginalDate = releaseGroup.FirstRelease
		newOriginalYear = extractYear(releaseGroup.FirstRelease)
		releaseGroupGenres = releaseGroup.Genres
	}

	artists := byTrack(func(t *core.FileTags) string { return t.Artist },
		func(track *core.MBTrack) string { return core.TrackArtist(release, track) })
	titles := byTrack(func(t *core.FileTags) string { return t.Title },
		func(track *core.MBTrack) string { return track.Title })
	medias := byTrack(func(t *core.FileTags) string { return t.Media },
		func(track *core.MBTrack) string { return track.MediumFormat })
	artistIDs := byTrack(func(t *core.FileTags) string { return t.MBArtistID },
		func(track *core.MBTrack) string { return core.TrackArtistID(release, track) })
	releaseIDs := tagdiff.Compare(files, func(t *core.FileTags) string { return t.MBReleaseID }, tagdiff.Every(release.ID))

	m.tagDiffs = []TagDiff{
		// Basic tags
		{Field: "Artist", OldValue: show(artists.Held), NewValue: show(artists.Written), Changed: artists.Changed},
		byRelease("Album Artist", func(t *core.FileTags) string { return t.AlbumArtist }, release.Artist),
		byRelease("Album", func(t *core.FileTags) string { return t.Album }, release.Title),
		{Field: "Track Titles", OldValue: "(see files)", NewValue: "(from MusicBrainz)", Changed: titles.Changed},

		// Date tags
		byRelease("Date", func(t *core.FileTags) string { return t.Date }, release.Date),
		byRelease("Original Date", func(t *core.FileTags) string { return t.OriginalDate }, newOriginalDate),
		byRelease("Original Year", func(t *core.FileTags) string { return t.OriginalYear() }, newOriginalYear),

		// Genre
		byRelease("Genre", func(t *core.FileTags) string { return t.Genre }, core.BuildGenreString(release.Genres, releaseGroupGenres)),

		// Release info
		byRelease("Label", func(t *core.FileTags) string { return t.Label }, release.Label),
		byRelease("Catalog #", func(t *core.FileTags) string { return t.CatalogNumber }, release.CatalogNumber),
		byRelease("Barcode", func(t *core.FileTags) string { return t.Barcode }, release.Barcode),
		{Field: "Media", OldValue: show(medias.Held), NewValue: show(medias.Written), Changed: medias.Changed},
		byRelease("Release Type", func(t *core.FileTags) string { return t.ReleaseType }, core.BuildReleaseType(releaseGroup)),
		byRelease("Status", func(t *core.FileTags) string { return t.ReleaseStatus }, release.Status),
		byRelease("Country", func(t *core.FileTags) string { return t.Country }, release.Country),
		byRelease("Script", func(t *core.FileTags) string { return t.Script }, release.Script),

		// MusicBrainz IDs (abbreviated for display, compared whole)
		{Field: "MB Artist ID", OldValue: showID(artistIDs.Held), NewValue: showID(artistIDs.Written), Changed: artistIDs.Changed},
		{Field: "MB Release ID", OldValue: showID(releaseIDs.Held), NewValue: truncateID(release.ID), Changed: releaseIDs.Changed},
	}
}

// show is how the preview shows the values of a tag on files: emptyValue when
// none has one, the value when they all hold the same, how many different
// ones they hold otherwise, an empty one counted.
func show(values tagdiff.Values) string {
	switch {
	case values.Several():
		return fmt.Sprintf("(%d different)", values.Distinct)
	case values.Value == "":
		return emptyValue
	default:
		return values.Value
	}
}

// showID is show for an ID, abbreviated.
func showID(values tagdiff.Values) string {
	if values.Several() || values.Value == "" {
		return show(values)
	}
	return truncateID(values.Value)
}

// truncateID truncates a UUID for display (first 8 chars).
func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}

func extractYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return date
}
