// internal/app/persistence.go
package app

import (
	"strconv"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/core"
)

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
		AlbumSelectedID:      m.Navigation.AlbumView().SelectedID(),
		AlbumGroupFields:     albumGroupFields,
		AlbumSortCriteria:    albumSortCriteria,
		BrowserSelectedState: browserState,
	})
}
