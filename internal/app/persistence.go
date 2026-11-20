// internal/app/persistence.go
package app

import (
	"strconv"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/core"
)

// saveNavigationAtQuit saves where the views are as waves quits: a view can
// have moved on its own since the user last navigated (a retag that renamed
// the album under the cursor, a refresh). It saves nothing before the
// initialization is applied, whose views hold nothing of the saved state, nor
// once the core is closing.
func (m *Model) saveNavigationAtQuit() {
	if !m.loadingInitDone || m.closing {
		return
	}
	m.SaveNavigationState()
}

// SaveNavigationState persists the current navigation state.
func (m *Model) SaveNavigationState() {
	// Convert library sub-mode to string
	var subMode string
	switch m.Navigation.LibrarySubMode() {
	case navctl.LibraryModeAlbum:
		subMode = "album"
	case navctl.LibraryModeMiller:
		subMode = "miller"
	case navctl.LibraryModeBrowser:
		subMode = "browser"
	}

	// Serialize album view settings
	albumGroupFields, albumSortCriteria, _ := m.Navigation.AlbumView().Settings().ToJSON()

	// Serialize browser selection state: "column\x00artist\x00album\x00trackID"
	var browserState string
	browser := m.Navigation.LibraryBrowser()
	artist := browser.SelectedArtistName()
	if artist != "" {
		albumName := ""
		if album := browser.SelectedAlbum(); album != nil {
			albumName = album.Name
		}
		trackID := ""
		if track := browser.SelectedTrack(); track != nil {
			trackID = strconv.FormatInt(track.ID, 10)
		}
		col := strconv.Itoa(int(browser.ActiveColumn()))
		browserState = col + "\x00" + artist + "\x00" + albumName + "\x00" + trackID
	}

	m.Core.ClientState().SaveNavigation(core.NavigationState{
		CurrentPath:          m.Navigation.FileNav().CurrentPath(),
		SelectedName:         m.Navigation.FileNav().SelectedName(),
		ViewMode:             string(m.Navigation.ViewMode()),
		LibrarySelectedID:    m.Navigation.LibraryNav().SelectedID(),
		PlaylistsSelectedID:  m.Navigation.PlaylistNav().SelectedID(),
		LibrarySubMode:       subMode,
		AlbumSelectedID:      m.Navigation.AlbumView().SavedSelection(),
		AlbumGroupFields:     albumGroupFields,
		AlbumSortCriteria:    albumSortCriteria,
		BrowserSelectedState: browserState,
	})
}
