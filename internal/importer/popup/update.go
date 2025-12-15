package popup

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/importer"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/rename"
)

const emptyValue = "(empty)"

// Init initializes the import popup and starts reading tags.
func (m *Model) Init() tea.Cmd {
	return ReadTagsCmd(m.completedPath, m.download)
}

// Update handles messages and key presses.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case TagsReadMsg:
		return m.handleTagsRead(msg)
	case MBReleaseRefreshedMsg:
		return m.handleReleaseRefreshed(msg)
	case FileImportedMsg:
		return m.handleFileImported(msg)
	case LibraryRefreshedMsg:
		return m.handleLibraryRefreshed(msg)
	}
	return m, nil
}

// handleKey handles key presses based on current state.
func (m *Model) handleKey(msg tea.KeyMsg) (*Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		return m.handleEscape()
	case "enter":
		return m.handleEnter()
	case "j", "down":
		return m.handleDown()
	case "k", "up":
		return m.handleUp()
	}

	return m, nil
}

// handleEscape handles the escape key - goes back or closes.
func (m *Model) handleEscape() (*Model, tea.Cmd) {
	switch m.state {
	case StateTagPreview:
		// Close popup
		return m, func() tea.Msg { return CloseMsg{} }
	case StatePathPreview:
		// Go back to tag preview
		m.state = StateTagPreview
		return m, nil
	case StateImporting:
		// Allow closing during import (import may continue in background but popup closes)
		return m, func() tea.Msg { return CloseMsg{} }
	case StateComplete:
		// Close popup
		return m, func() tea.Msg { return CloseMsg{} }
	}
	return m, nil
}

// handleEnter handles the enter key - proceeds to next step.
func (m *Model) handleEnter() (*Model, tea.Cmd) {
	switch m.state {
	case StateTagPreview:
		// Proceed to path preview
		m.state = StatePathPreview
		m.buildPathMappings()
		return m, nil
	case StatePathPreview:
		// Start import
		m.state = StateImporting
		cmd := m.startImport()
		return m, cmd
	case StateImporting:
		// Can't interact during import
		return m, nil
	case StateComplete:
		// Close popup
		return m, func() tea.Msg { return CloseMsg{} }
	}
	return m, nil
}

// handleDown handles down/j key.
func (m *Model) handleDown() (*Model, tea.Cmd) {
	switch m.state {
	case StateTagPreview, StateImporting, StateComplete:
		// No scrolling in these states
		return m, nil
	case StatePathPreview:
		if len(m.librarySources) > 1 {
			m.selectedSource = (m.selectedSource + 1) % len(m.librarySources)
			m.buildPathMappings()
		}
	}
	return m, nil
}

// handleUp handles up/k key.
func (m *Model) handleUp() (*Model, tea.Cmd) {
	switch m.state {
	case StateTagPreview, StateImporting, StateComplete:
		// No scrolling in these states
		return m, nil
	case StatePathPreview:
		if len(m.librarySources) > 1 {
			m.selectedSource--
			if m.selectedSource < 0 {
				m.selectedSource = len(m.librarySources) - 1
			}
			m.buildPathMappings()
		}
	}
	return m, nil
}

// handleTagsRead handles the result of reading tags from files.
func (m *Model) handleTagsRead(msg TagsReadMsg) (*Model, tea.Cmd) {
	if msg.Err != nil {
		// Still continue with empty tags
		m.currentTags = make([]player.TrackInfo, len(m.download.Files))
	} else {
		m.currentTags = msg.Tags
	}

	// Check if we need to refresh MusicBrainz data
	// 1. Look for MBReleaseID in the downloaded files
	// 2. If it differs from the current selection, switch to that release
	// 3. If same or no ID found, check if extended fields are missing and refresh
	fileReleaseID := m.findReleaseIDFromFiles()
	currentReleaseID := ""
	if m.download.MBReleaseDetails != nil {
		currentReleaseID = m.download.MBReleaseDetails.ID
	}

	needsRefresh := false
	targetReleaseID := currentReleaseID

	if fileReleaseID != "" && fileReleaseID != currentReleaseID {
		// Files have a different release ID - switch to that release
		needsRefresh = true
		targetReleaseID = fileReleaseID
	} else if m.download.MBReleaseDetails != nil && m.releaseNeedsRefresh() {
		// Same release but missing extended fields - refresh
		needsRefresh = true
	}

	if needsRefresh && targetReleaseID != "" && m.mbClient != nil {
		m.loadingMB = true
		return m, RefreshReleaseCmd(m.mbClient, targetReleaseID, currentReleaseID)
	}

	// Build tag diffs
	m.buildTagDiffs()

	return m, nil
}

// findReleaseIDFromFiles looks for a MusicBrainz release ID in the downloaded files.
// Returns the first non-empty ID found, or empty string if none found.
func (m *Model) findReleaseIDFromFiles() string {
	for i := range m.currentTags {
		if m.currentTags[i].MBReleaseID != "" {
			return m.currentTags[i].MBReleaseID
		}
	}
	return ""
}

// releaseNeedsRefresh checks if the current release is missing extended fields.
func (m *Model) releaseNeedsRefresh() bool {
	release := m.download.MBReleaseDetails
	if release == nil {
		return false
	}

	// Check if important extended fields are missing
	// These fields should have been populated by the new extraction code
	// If they're empty, we need to re-fetch
	return release.ArtistID == "" || release.Label == "" && release.Status == ""
}

// handleReleaseRefreshed handles the MusicBrainz release data refresh completion.
func (m *Model) handleReleaseRefreshed(msg MBReleaseRefreshedMsg) (*Model, tea.Cmd) {
	m.loadingMB = false

	if msg.Err != nil {
		// Refresh failed, continue with existing data
		m.buildTagDiffs()
		return m, nil
	}

	if msg.Release != nil {
		// Update the download's release details with the fresh data
		m.download.MBReleaseDetails = msg.Release
	}

	// Build tag diffs with the updated release data
	m.buildTagDiffs()

	return m, nil
}

// handleFileImported handles the result of importing a single file.
func (m *Model) handleFileImported(msg FileImportedMsg) (*Model, tea.Cmd) {
	if msg.Index < 0 || msg.Index >= len(m.importStatus) {
		return m, nil
	}

	if msg.Err != nil {
		m.importStatus[msg.Index].Status = StatusFailed
		m.importStatus[msg.Index].Error = msg.Err.Error()
		m.failedFiles = append(m.failedFiles, FailedFile{
			Filename: m.importStatus[msg.Index].Filename,
			Error:    msg.Err.Error(),
		})
	} else {
		m.importStatus[msg.Index].Status = StatusComplete
		m.successCount++
		// Track successfully imported path
		if msg.DestPath != "" {
			m.importedPaths = append(m.importedPaths, msg.DestPath)
		}
	}

	// Check if all files are done
	allDone := true
	for _, s := range m.importStatus {
		if s.Status == StatusPending || s.Status == StatusTagging || s.Status == StatusMoving {
			allDone = false
			break
		}
	}

	if allDone {
		// Import cover art if we have successful imports
		if len(m.importedPaths) > 0 {
			// Source directory is where the downloaded files are
			sourceDir := downloads.BuildDiskPath(m.completedPath, m.download.SlskdDirectory)
			// Destination directory is the album folder (parent of any imported track)
			destDir := filepath.Dir(m.importedPaths[0])
			// Import cover art (move mode, ignore errors)
			_, _ = importer.ImportCoverArt(sourceDir, destDir, false)
		}

		// All done, signal completion with navigation info
		artistName := ""
		albumName := ""
		if m.download.MBReleaseDetails != nil {
			artistName = m.download.MBReleaseDetails.Artist
			albumName = m.download.MBReleaseDetails.Title
		}

		return m, func() tea.Msg {
			return ImportCompleteMsg{
				SuccessCount:  m.successCount,
				FailedFiles:   m.failedFiles,
				DownloadID:    m.download.ID,
				ArtistName:    artistName,
				AlbumName:     albumName,
				AllSucceeded:  len(m.failedFiles) == 0,
				ImportedPaths: m.importedPaths,
			}
		}
	}

	// Import next file
	cmd := m.importNextFile()
	return m, cmd
}

// handleLibraryRefreshed handles the library refresh completion.
func (m *Model) handleLibraryRefreshed(_ LibraryRefreshedMsg) (*Model, tea.Cmd) {
	// Move to complete state regardless of error
	m.state = StateComplete
	return m, nil
}

// buildTagDiffs builds the tag comparison data.
func (m *Model) buildTagDiffs() {
	m.tagDiffs = nil

	if m.download.MBReleaseDetails == nil {
		return
	}

	release := m.download.MBReleaseDetails
	releaseGroup := m.download.MBReleaseGroup

	// Collect current values from files (basic tags)
	artists := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.Artist })
	albumArtists := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.AlbumArtist })
	albums := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.Album })
	years := collectValues(m.currentTags, func(t player.TrackInfo) string {
		if t.Year == 0 {
			return ""
		}
		return strconv.Itoa(t.Year)
	})
	genres := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.Genre })

	// Collect current values from files (extended tags)
	dates := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.Date })
	originalDates := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.OriginalDate })
	originalYears := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.OriginalYear })
	labels := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.Label })
	catalogNums := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.CatalogNumber })
	barcodes := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.Barcode })
	medias := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.Media })
	releaseTypes := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.ReleaseType })
	releaseStatuses := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.ReleaseStatus })
	countries := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.Country })
	scripts := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.Script })
	mbArtistIDs := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.MBArtistID })
	mbReleaseIDs := collectValues(m.currentTags, func(t player.TrackInfo) string { return t.MBReleaseID })

	// New values from MusicBrainz
	newDate := release.Date
	newOriginalDate := ""
	newOriginalYear := ""
	if releaseGroup != nil && releaseGroup.FirstRelease != "" {
		newOriginalDate = releaseGroup.FirstRelease
		newOriginalYear = extractYear(releaseGroup.FirstRelease)
	}

	// Build genre string (multiple genres with ";" separator, title-cased)
	newGenre := buildGenreString(release.Genres, releaseGroup)

	// Build release type string
	newReleaseType := ""
	if releaseGroup != nil {
		newReleaseType = strings.ToLower(releaseGroup.PrimaryType)
		if len(releaseGroup.SecondaryTypes) > 0 {
			newReleaseType += "; " + strings.Join(releaseGroup.SecondaryTypes, "; ")
		}
	}

	// Build diffs - showing all important tags with actual old values
	m.tagDiffs = []TagDiff{
		// Basic tags
		{Field: "Artist", OldValue: formatMultiValue(artists), NewValue: release.Artist, Changed: !allMatch(artists, release.Artist)},
		{Field: "Album Artist", OldValue: formatMultiValue(albumArtists), NewValue: release.Artist, Changed: !allMatch(albumArtists, release.Artist)},
		{Field: "Album", OldValue: formatMultiValue(albums), NewValue: release.Title, Changed: !allMatch(albums, release.Title)},
		{Field: "Track Titles", OldValue: "(see files)", NewValue: "(from MusicBrainz)", Changed: true},

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
		{Field: "MB Artist ID", OldValue: formatMultiValueTruncated(mbArtistIDs), NewValue: truncateID(release.ArtistID), Changed: !allMatchTruncated(mbArtistIDs, release.ArtistID)},
		{Field: "MB Release ID", OldValue: formatMultiValueTruncated(mbReleaseIDs), NewValue: truncateID(release.ID), Changed: !allMatchTruncated(mbReleaseIDs, release.ID)},
	}
}

// buildGenreString builds a semicolon-separated genre string.
func buildGenreString(releaseGenres []string, releaseGroup *musicbrainz.ReleaseGroup) string {
	genres := releaseGenres
	if len(genres) == 0 && releaseGroup != nil {
		genres = releaseGroup.Genres
	}
	if len(genres) == 0 {
		return ""
	}

	// Title-case each genre
	titleCased := make([]string, len(genres))
	for i, g := range genres {
		titleCased[i] = titleCase(g)
	}
	return strings.Join(titleCased, ";")
}

// titleCase converts a string to title case.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// truncateID truncates a UUID for display (first 8 chars).
func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8] + "..."
	}
	return id
}

// buildPathMappings builds the path mapping data.
func (m *Model) buildPathMappings() {
	m.filePaths = nil

	if m.download.MBReleaseDetails == nil || len(m.librarySources) == 0 {
		return
	}

	destRoot := m.librarySources[m.selectedSource]

	// Sort files by track number
	sortedFiles := downloads.SortFilesByTrackNumber(m.download.Files)

	for i, f := range sortedFiles {
		trackNum := downloads.ParseTrackNumber(f.Filename)
		if trackNum == 0 {
			trackNum = i + 1
		}

		// Build source path
		normalizedFilename := strings.ReplaceAll(f.Filename, "\\", "/")
		filename := filepath.Base(normalizedFilename)

		// Find matching MB track by index
		trackIndex := i
		if trackIndex >= len(m.download.MBReleaseDetails.Tracks) {
			trackIndex = len(m.download.MBReleaseDetails.Tracks) - 1
		}

		// Build destination path
		newPath := m.buildDestPathForTrack(destRoot, trackIndex, filepath.Ext(filename))

		m.filePaths = append(m.filePaths, PathMapping{
			TrackNum: trackNum,
			OldPath:  BuildSourcePath(m.completedPath, m.download, &f),
			NewPath:  newPath,
			Filename: filename,
		})
	}
}

// buildDestPathForTrack builds the destination path for a track.
func (m *Model) buildDestPathForTrack(destRoot string, trackIndex int, ext string) string {
	if m.download.MBReleaseDetails == nil || trackIndex >= len(m.download.MBReleaseDetails.Tracks) {
		return ""
	}

	track := m.download.MBReleaseDetails.Tracks[trackIndex]

	// Build metadata for renaming
	releaseType := ""
	secondaryTypes := ""
	originalDate := ""
	if m.download.MBReleaseGroup != nil {
		releaseType = strings.ToLower(m.download.MBReleaseGroup.PrimaryType)
		secondaryTypes = strings.Join(m.download.MBReleaseGroup.SecondaryTypes, "; ")
		originalDate = m.download.MBReleaseGroup.FirstRelease
	}

	// Get disc info from release and track
	discNumber := track.DiscNumber
	if discNumber == 0 {
		discNumber = 1
	}
	totalDiscs := m.download.MBReleaseDetails.DiscCount
	if totalDiscs == 0 {
		totalDiscs = 1
	}

	meta := rename.TrackMetadata{
		Artist:               m.download.MBReleaseDetails.Artist,
		AlbumArtist:          m.download.MBReleaseDetails.Artist,
		Album:                m.download.MBReleaseDetails.Title,
		Title:                track.Title,
		TrackNumber:          track.Position,
		DiscNumber:           discNumber,
		TotalDiscs:           totalDiscs,
		Date:                 m.download.MBReleaseDetails.Date,
		OriginalDate:         originalDate,
		ReleaseType:          releaseType,
		SecondaryReleaseType: secondaryTypes,
	}

	relPath := rename.GeneratePath(meta)
	return filepath.Join(destRoot, relPath+ext)
}

// startImport starts the import process.
func (m *Model) startImport() tea.Cmd {
	// Start importing the first file directly
	if len(m.importStatus) > 0 {
		m.importStatus[0].Status = StatusTagging
		m.currentFile = 0
		return m.importFile(0)
	}
	return nil
}

// importNextFile imports the next pending file.
func (m *Model) importNextFile() tea.Cmd {
	// Find next pending file
	for i, s := range m.importStatus {
		if s.Status == StatusPending {
			m.importStatus[i].Status = StatusTagging
			m.currentFile = i
			return m.importFile(i)
		}
	}
	// No more pending files - this should trigger completion check in handleFileImported
	return nil
}

// importFile creates a command to import a single file.
func (m *Model) importFile(index int) tea.Cmd {
	// Handle edge cases that would cause a hang
	if index >= len(m.filePaths) {
		return func() tea.Msg {
			return FileImportedMsg{
				Index: index,
				Err:   fmt.Errorf("file index %d out of range (paths: %d)", index, len(m.filePaths)),
			}
		}
	}

	if m.download.MBReleaseDetails == nil {
		return func() tea.Msg {
			return FileImportedMsg{
				Index: index,
				Err:   errors.New("no release details available"),
			}
		}
	}

	pm := m.filePaths[index]

	// Find matching track index
	trackIndex := index
	if trackIndex >= len(m.download.MBReleaseDetails.Tracks) {
		trackIndex = len(m.download.MBReleaseDetails.Tracks) - 1
	}

	destRoot := ""
	if len(m.librarySources) > 0 {
		destRoot = m.librarySources[m.selectedSource]
	}

	// Get disc info from release and track
	track := m.download.MBReleaseDetails.Tracks[trackIndex]
	discNumber := track.DiscNumber
	if discNumber == 0 {
		discNumber = 1
	}
	totalDiscs := m.download.MBReleaseDetails.DiscCount
	if totalDiscs == 0 {
		totalDiscs = 1
	}

	return func() tea.Msg {
		result, err := importer.Import(importer.ImportParams{
			SourcePath:   pm.OldPath,
			DestRoot:     destRoot,
			ReleaseGroup: m.download.MBReleaseGroup,
			Release:      m.download.MBReleaseDetails,
			TrackIndex:   trackIndex,
			DiscNumber:   discNumber,
			TotalDiscs:   totalDiscs,
			CoverArt:     nil, // TODO: fetch cover art
			CopyMode:     false,
		})

		if err != nil {
			return FileImportedMsg{Index: index, Err: err}
		}
		return FileImportedMsg{Index: index, DestPath: result.DestPath}
	}
}

// Helper functions

func collectValues(tags []player.TrackInfo, getter func(player.TrackInfo) string) []string {
	seen := make(map[string]bool)
	var values []string
	for i := range tags {
		v := getter(tags[i])
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
	if len(values) == 1 {
		if values[0] == "" {
			return emptyValue
		}
		return values[0]
	}
	return fmt.Sprintf("(%d different)", len(values))
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
	if len(nonEmpty) == 1 {
		return truncateID(nonEmpty[0])
	}
	return fmt.Sprintf("(%d different)", len(nonEmpty))
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

// GetReleaseGroup returns the release group (needed for import params).
func (m *Model) GetReleaseGroup() *musicbrainz.ReleaseGroup {
	return m.download.MBReleaseGroup
}

// GetReleaseDetails returns the release details (needed for import params).
func (m *Model) GetReleaseDetails() *musicbrainz.ReleaseDetails {
	return m.download.MBReleaseDetails
}
