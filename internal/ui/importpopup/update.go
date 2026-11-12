package importpopup

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	uipopup "github.com/llehouerou/waves/internal/ui/popup"
)

// Compile-time check that Model implements popup.Popup.
var _ uipopup.Popup = (*Model)(nil)

const emptyValue = "(empty)"

// Init asks the core for the plan of the import.
func (m *Model) Init() tea.Cmd {
	return prepareCmd(m.importer, m.download.ID)
}

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (uipopup.Popup, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case PreparedMsg:
		if msg.DownloadID == m.download.ID {
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
		m.moveSource(1)
	case "k", "up":
		m.moveSource(-1)
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

// moveSource moves the library source cursor of the path preview.
func (m *Model) moveSource(delta int) {
	if n := len(m.librarySources); m.state == StatePathPreview && n > 1 {
		m.selectedSource = (m.selectedSource + delta + n) % n
	}
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
	m.buildTagDiffs()
}

// startImport asks the core to import the plan into the selected library
// source. The core returns at once: its events tell the rest.
func (m *Model) startImport() {
	if len(m.librarySources) == 0 {
		return
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

// buildTagDiffs builds the tag comparison data.
func (m *Model) buildTagDiffs() {
	release := m.plan.Release
	releaseGroup := m.plan.ReleaseGroup

	currentTags := make([]core.FileTags, len(m.plan.Files))
	for i := range m.plan.Files {
		currentTags[i] = m.plan.Files[i].Tags
	}

	// Collect current values from files (basic tags)
	artists := collectValues(currentTags, func(t core.FileTags) string { return t.Artist })
	albumArtists := collectValues(currentTags, func(t core.FileTags) string { return t.AlbumArtist })
	albums := collectValues(currentTags, func(t core.FileTags) string { return t.Album })

	// Each file's MusicBrainz track: the preview shows what the import will write
	matched := make([]*core.MBTrack, len(m.plan.Files))
	titlesChanged := false
	for i := range m.plan.Files {
		if trackIndex := m.plan.Files[i].Track; trackIndex >= 0 {
			matched[i] = &release.Tracks[trackIndex]
		}
		if matched[i] == nil || currentTags[i].Title != matched[i].Title {
			titlesChanged = true
		}
	}
	years := collectValues(currentTags, func(t core.FileTags) string {
		if t.Year() == 0 {
			return ""
		}
		return strconv.Itoa(t.Year())
	})
	genres := collectValues(currentTags, func(t core.FileTags) string { return t.Genre })

	// Collect current values from files (extended tags)
	dates := collectValues(currentTags, func(t core.FileTags) string { return t.Date })
	originalDates := collectValues(currentTags, func(t core.FileTags) string { return t.OriginalDate })
	originalYears := collectValues(currentTags, func(t core.FileTags) string { return t.OriginalYear() })
	labels := collectValues(currentTags, func(t core.FileTags) string { return t.Label })
	catalogNums := collectValues(currentTags, func(t core.FileTags) string { return t.CatalogNumber })
	barcodes := collectValues(currentTags, func(t core.FileTags) string { return t.Barcode })
	medias := collectValues(currentTags, func(t core.FileTags) string { return t.Media })
	releaseTypes := collectValues(currentTags, func(t core.FileTags) string { return t.ReleaseType })
	releaseStatuses := collectValues(currentTags, func(t core.FileTags) string { return t.ReleaseStatus })
	countries := collectValues(currentTags, func(t core.FileTags) string { return t.Country })
	scripts := collectValues(currentTags, func(t core.FileTags) string { return t.Script })
	mbArtistIDs := collectValues(currentTags, func(t core.FileTags) string { return t.MBArtistID })
	mbReleaseIDs := collectValues(currentTags, func(t core.FileTags) string { return t.MBReleaseID })

	// New values from MusicBrainz
	newDate := release.Date
	newOriginalDate := ""
	newOriginalYear := ""
	if releaseGroup != nil && releaseGroup.FirstRelease != "" {
		newOriginalDate = releaseGroup.FirstRelease
		newOriginalYear = extractYear(releaseGroup.FirstRelease)
	}

	// Build genre string (multiple genres with ";" separator, title-cased)
	var releaseGroupGenres []string
	if releaseGroup != nil {
		releaseGroupGenres = releaseGroup.Genres
	}
	newGenre := core.BuildGenreString(release.Genres, releaseGroupGenres)

	// Build release type string
	newReleaseType := ""
	if releaseGroup != nil {
		newReleaseType = strings.ToLower(releaseGroup.PrimaryType)
		if len(releaseGroup.SecondaryTypes) > 0 {
			newReleaseType += "; " + strings.Join(releaseGroup.SecondaryTypes, "; ")
		}
	}

	// Collect new artist values from the files' tracks (use track artist if set, else album artist)
	newArtists := make([]string, 0, len(matched))
	newArtistIDs := make([]string, 0, len(matched))
	for _, track := range matched {
		if track == nil {
			continue
		}
		artist := track.Artist
		if artist == "" {
			artist = release.Artist
		}
		newArtists = append(newArtists, artist)

		artistID := track.ArtistID
		if artistID == "" {
			artistID = release.ArtistID
		}
		newArtistIDs = append(newArtistIDs, artistID)
	}
	newArtistDisplay := formatMultiValue(newArtists)
	newArtistIDDisplay := formatMultiValueTruncated(newArtistIDs)

	// Build diffs - showing all important tags with actual old values
	m.tagDiffs = []TagDiff{
		// Basic tags
		{Field: "Artist", OldValue: formatMultiValue(artists), NewValue: newArtistDisplay, Changed: !slicesEqualUnique(artists, newArtists)},
		{Field: "Album Artist", OldValue: formatMultiValue(albumArtists), NewValue: release.Artist, Changed: !allMatch(albumArtists, release.Artist)},
		{Field: "Album", OldValue: formatMultiValue(albums), NewValue: release.Title, Changed: !allMatch(albums, release.Title)},
		{Field: "Track Titles", OldValue: "(see files)", NewValue: "(from MusicBrainz)", Changed: titlesChanged},

		// Date tags
		{Field: "Date", OldValue: formatMultiValueOrYear(dates, years), NewValue: newDate, Changed: !allMatch(dates, newDate)},
		{Field: "Original Date", OldValue: formatMultiValue(originalDates), NewValue: newOriginalDate, Changed: !allMatch(originalDates, newOriginalDate)},
		{Field: "Original Year", OldValue: formatMultiValue(originalYears), NewValue: newOriginalYear, Changed: !allMatch(originalYears, newOriginalYear)},

		// Genre
		{Field: "Genre", OldValue: formatMultiValue(genres), NewValue: newGenre, Changed: !allMatch(genres, newGenre)},

		// Release info
		{Field: "Label", OldValue: formatMultiValue(labels), NewValue: release.Label, Changed: !allMatch(labels, release.Label)},
		{Field: "Catalog #", OldValue: formatMultiValue(catalogNums), NewValue: release.CatalogNumber, Changed: !allMatch(catalogNums, release.CatalogNumber)},
		{Field: "Barcode", OldValue: formatMultiValue(barcodes), NewValue: release.Barcode, Changed: !allMatch(barcodes, release.Barcode)},
		{Field: "Media", OldValue: formatMultiValue(medias), NewValue: release.Formats, Changed: !allMatch(medias, release.Formats)},
		{Field: "Release Type", OldValue: formatMultiValue(releaseTypes), NewValue: newReleaseType, Changed: !allMatch(releaseTypes, newReleaseType)},
		{Field: "Status", OldValue: formatMultiValue(releaseStatuses), NewValue: release.Status, Changed: !allMatch(releaseStatuses, release.Status)},
		{Field: "Country", OldValue: formatMultiValue(countries), NewValue: release.Country, Changed: !allMatch(countries, release.Country)},
		{Field: "Script", OldValue: formatMultiValue(scripts), NewValue: release.Script, Changed: !allMatch(scripts, release.Script)},

		// MusicBrainz IDs (abbreviated for display)
		{Field: "MB Artist ID", OldValue: formatMultiValueTruncated(mbArtistIDs), NewValue: newArtistIDDisplay, Changed: !slicesEqualUnique(mbArtistIDs, newArtistIDs)},
		{Field: "MB Release ID", OldValue: formatMultiValueTruncated(mbReleaseIDs), NewValue: truncateID(release.ID), Changed: !allMatchTruncated(mbReleaseIDs, release.ID)},
	}
}

// truncateID truncates a UUID for display (first 8 chars).
func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}

// Helper functions

func collectValues(files []core.FileTags, getter func(core.FileTags) string) []string {
	seen := make(map[string]bool)
	var values []string
	for i := range files {
		v := getter(files[i])
		if !seen[v] {
			seen[v] = true
			values = append(values, v)
		}
	}
	return values
}

func formatMultiValue(values []string) string {
	if len(values) == 0 {
		return emptyValue
	}
	// Deduplicate values
	unique := dedupeStrings(values)
	if len(unique) == 1 {
		if unique[0] == "" {
			return emptyValue
		}
		return unique[0]
	}
	return fmt.Sprintf("(%d different)", len(unique))
}

// formatMultiValueOrYear returns the date values, or falls back to year values if dates are empty.
func formatMultiValueOrYear(dates, years []string) string {
	// Try dates first
	nonEmpty := filterNonEmpty(dates)
	if len(nonEmpty) > 0 {
		return formatMultiValue(nonEmpty)
	}
	// Fall back to years
	return formatMultiValue(years)
}

// formatMultiValueTruncated formats multiple values with truncation for IDs.
func formatMultiValueTruncated(values []string) string {
	nonEmpty := filterNonEmpty(values)
	if len(nonEmpty) == 0 {
		return emptyValue
	}
	// Deduplicate values
	unique := dedupeStrings(nonEmpty)
	if len(unique) == 1 {
		return truncateID(unique[0])
	}
	return fmt.Sprintf("(%d different)", len(unique))
}

// dedupeStrings returns unique strings preserving order.
func dedupeStrings(values []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// filterNonEmpty returns only non-empty strings from the slice.
func filterNonEmpty(values []string) []string {
	var result []string
	for _, v := range values {
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}

func allMatch(values []string, target string) bool {
	if len(values) == 0 {
		return target == ""
	}
	for _, v := range values {
		if v != target {
			return false
		}
	}
	return true
}

// slicesEqualUnique returns true if two string slices have the same unique non-empty values.
func slicesEqualUnique(a, b []string) bool {
	uniqueA := make(map[string]bool)
	for _, v := range a {
		if v != "" {
			uniqueA[v] = true
		}
	}
	uniqueB := make(map[string]bool)
	for _, v := range b {
		if v != "" {
			uniqueB[v] = true
		}
	}
	if len(uniqueA) != len(uniqueB) {
		return false
	}
	for k := range uniqueA {
		if !uniqueB[k] {
			return false
		}
	}
	return true
}

// allMatchTruncated checks if all values match the target (comparing full values, not truncated).
func allMatchTruncated(values []string, target string) bool {
	nonEmpty := filterNonEmpty(values)
	if len(nonEmpty) == 0 {
		return target == ""
	}
	for _, v := range nonEmpty {
		if v != target {
			return false
		}
	}
	return true
}

func extractYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return date
}
