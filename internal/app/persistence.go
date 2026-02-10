// internal/app/persistence.go
package app

import (
	"strconv"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/state"
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

	m.StateMgr.SaveNavigation(state.NavigationState{
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

// SaveQueueState persists the current queue state.
func (m *Model) SaveQueueState() {
	tracks := m.PlaybackService.QueueTracks()
	queueTracks := make([]state.QueueTrack, len(tracks))
	for i, t := range tracks {
		queueTracks[i] = state.QueueTrack{
			TrackID:     t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
		}
	}
	_ = m.StateMgr.SaveQueue(state.QueueState{
		CurrentIndex: m.PlaybackService.QueueCurrentIndex(),
		RepeatMode:   int(m.PlaybackService.RepeatMode()),
		Shuffle:      m.PlaybackService.Shuffle(),
		Tracks:       queueTracks,
	})
}

// TracksToQueueTracks converts playlist tracks to state queue tracks.
func TracksToQueueTracks(tracks []playlist.Track) []state.QueueTrack {
	result := make([]state.QueueTrack, len(tracks))
	for i, t := range tracks {
		result[i] = state.QueueTrack{
			TrackID:     t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
		}
	}
	return result
}
