// internal/app/handlers_library.go
package app

import (
	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
)

// handleLibraryKeys handles library-specific keys (d for delete, f for favorite, V for album view).
func (m *Model) handleLibraryKeys(key string) handler.Result {
	if m.Navigation.ViewMode() != navctl.ViewLibrary || !m.Navigation.IsNavigatorFocused() {
		return handler.NotHandled
	}

	action := m.Keys.Resolve(key)

	// V toggles album view sub-mode (works regardless of selection)
	if action == keymap.ActionToggleAlbumView {
		m.toggleLibraryViewMode()
		return handler.HandledNoCmd
	}

	// t opens retag popup (works in both Miller columns and Album view)
	if action == keymap.ActionRetag {
		return m.handleRetagKey()
	}

	// i opens similar artists popup (works in both Miller columns and Album view)
	if action == keymap.ActionSimilarArtists {
		return m.handleSimilarArtists()
	}

	// Browser mode: handle F and d keys at track level
	if m.Navigation.IsBrowserViewActive() {
		browser := m.Navigation.LibraryBrowser()
		switch action { //nolint:exhaustive // only handling library-specific actions
		case keymap.ActionToggleFavorite:
			if browser.ActiveColumn() != librarybrowser.ColumnTracks {
				return handler.NotHandled
			}
			track := browser.SelectedTrack()
			if track == nil {
				return handler.HandledNoCmd
			}
			return m.handleToggleFavorite([]int64{track.ID})

		case keymap.ActionDelete:
			if browser.ActiveColumn() != librarybrowser.ColumnTracks {
				return handler.NotHandled
			}
			track := browser.SelectedTrack()
			if track == nil {
				return handler.HandledNoCmd
			}
			m.Popups.ShowConfirmWithOptions(
				"Delete Track",
				"Delete \""+track.Title+"\"?",
				[]string{"Remove from library", "Delete from disk", "Cancel"},
				LibraryDeleteContext{
					TrackID:   track.ID,
					TrackPath: track.Path,
					Title:     track.Title,
				},
			)
			return handler.HandledNoCmd
		}
		return handler.NotHandled
	}

	// Album view doesn't use these keys - they're handled by the view itself
	if m.Navigation.LibrarySubMode() == navctl.LibraryModeAlbum {
		return handler.NotHandled
	}

	selected := m.Navigation.LibraryNav().Selected()
	if selected == nil {
		return handler.NotHandled
	}

	switch action { //nolint:exhaustive // only handling library-specific actions
	case keymap.ActionToggleFavorite:
		// Toggle favorite - only at track level
		if selected.Level() != library.LevelTrack {
			return handler.NotHandled
		}
		track := selected.Track()
		if track == nil {
			return handler.HandledNoCmd
		}
		return m.handleToggleFavorite([]int64{track.ID})

	case keymap.ActionDelete:
		// Delete track - only at track level
		if selected.Level() != library.LevelTrack {
			return handler.NotHandled
		}

		track := selected.Track()
		if track == nil {
			return handler.HandledNoCmd
		}

		m.Popups.ShowConfirmWithOptions(
			"Delete Track",
			"Delete \""+track.Title+"\"?",
			[]string{"Remove from library", "Delete from disk", "Cancel"},
			LibraryDeleteContext{
				TrackID:   track.ID,
				TrackPath: track.Path,
				Title:     track.Title,
			},
		)
		return handler.HandledNoCmd
	}

	return handler.NotHandled
}

// handleToggleFavorite toggles favorite status for the given track IDs.
func (m *Model) handleToggleFavorite(trackIDs []int64) handler.Result {
	if len(trackIDs) == 0 {
		return handler.HandledNoCmd
	}

	results, err := m.Playlists.ToggleFavorites(trackIDs)
	if err != nil {
		m.Popups.ShowOpError(errmsg.OpFavoriteToggle, err)
		return handler.HandledNoCmd
	}

	// Refresh favorites in navigators
	m.RefreshFavorites()

	// Refresh playlist navigator (Favorites playlist contents changed)
	// This ensures the Favorites playlist shows correct tracks when viewed
	m.refreshPlaylistNavigatorInPlace()

	_ = results // results used for refreshing, no message needed
	return handler.HandledNoCmd
}

// toggleLibraryViewMode switches between miller columns and album view,
// preserving the current album selection.
func (m *Model) toggleLibraryViewMode() {
	// Capture current album before switching
	albumArtist, albumName := m.getCurrentLibraryAlbum()

	m.Navigation.ToggleLibrarySubMode()
	m.Navigation.SetFocus(m.Navigation.Focus())

	// Select the same album in the new view
	m.selectAlbumInCurrentMode(albumArtist, albumName)
	m.SaveNavigationState()
}

// getCurrentLibraryAlbum returns the album artist and name from the current view.
func (m *Model) getCurrentLibraryAlbum() (artist, album string) {
	switch m.Navigation.LibrarySubMode() { //nolint:exhaustive // default handles Miller mode
	case navctl.LibraryModeAlbum:
		if a := m.Navigation.AlbumView().SelectedAlbum(); a != nil {
			return a.AlbumArtist, a.Album
		}
	case navctl.LibraryModeBrowser:
		browser := m.Navigation.LibraryBrowser()
		a := browser.SelectedArtist()
		if alb := browser.SelectedAlbum(); alb != nil {
			return a, alb.Name
		}
	default:
		if selected := m.Navigation.LibraryNav().Selected(); selected != nil {
			return selected.Artist(), selected.Album()
		}
	}
	return "", ""
}

// selectAlbumInCurrentMode selects the album in the current library sub-mode.
func (m *Model) selectAlbumInCurrentMode(albumArtist, albumName string) {
	switch m.Navigation.LibrarySubMode() { //nolint:exhaustive // default handles Miller mode
	case navctl.LibraryModeAlbum:
		if err := m.Navigation.AlbumView().Refresh(); err != nil {
			m.Popups.ShowOpError(errmsg.OpAlbumLoad, err)
			return
		}
		if albumArtist != "" && albumName != "" {
			m.Navigation.AlbumView().SelectByID(albumArtist + ":" + albumName)
		}
	case navctl.LibraryModeBrowser:
		if albumArtist != "" && albumName != "" {
			browser := m.Navigation.LibraryBrowser()
			browser.SelectArtist(albumArtist)
			browser.SelectAlbum(albumName)
		}
	default:
		if albumArtist != "" && albumName != "" {
			albumID := "library:album:" + albumArtist + ":" + albumName
			m.Navigation.LibraryNav().NavigateTo(albumID)
		}
	}
}

// handleRetagKey handles the 't' key to open the retag popup.
func (m *Model) handleRetagKey() handler.Result {
	// Get album info from current view mode
	var albumArtist, albumName string

	switch m.Navigation.LibrarySubMode() { //nolint:exhaustive // default handles Miller mode
	case navctl.LibraryModeAlbum:
		// Album view mode
		album := m.Navigation.AlbumView().SelectedAlbum()
		if album == nil {
			return handler.NotHandled
		}
		albumArtist = album.AlbumArtist
		albumName = album.Album
	case navctl.LibraryModeBrowser:
		// Browser mode - must be at album level
		browser := m.Navigation.LibraryBrowser()
		if browser.ActiveColumn() != librarybrowser.ColumnAlbums {
			return handler.NotHandled
		}
		albumArtist = browser.SelectedArtist()
		alb := browser.SelectedAlbum()
		if alb == nil {
			return handler.NotHandled
		}
		albumName = alb.Name
	default:
		// Miller columns mode - must be at album level
		selected := m.Navigation.LibraryNav().Selected()
		if selected == nil || selected.Level() != library.LevelAlbum {
			return handler.NotHandled
		}
		albumArtist = selected.Artist()
		albumName = selected.Album()
	}

	if albumArtist == "" || albumName == "" {
		return handler.NotHandled
	}

	// Get track paths for the album
	trackIDs, err := m.Library.AlbumTrackIDs(albumArtist, albumName)
	if err != nil || len(trackIDs) == 0 {
		m.Popups.ShowOpError(errmsg.OpAlbumLoad, err)
		return handler.HandledNoCmd
	}

	trackPaths := make([]string, 0, len(trackIDs))
	for _, id := range trackIDs {
		track, err := m.Library.TrackByID(id)
		if err != nil {
			continue
		}
		trackPaths = append(trackPaths, track.Path)
	}

	if len(trackPaths) == 0 {
		return handler.HandledNoCmd
	}

	// Open retag popup
	mbClient := musicbrainz.NewClient()
	cmd := m.Popups.ShowRetag(albumArtist, albumName, trackPaths, mbClient, m.Library)
	return handler.Handled(cmd)
}

// handleSimilarArtists opens the similar artists popup for the selected item.
func (m *Model) handleSimilarArtists() handler.Result {
	// Need Last.fm config
	if !m.HasLastfmConfig || m.Lastfm == nil {
		return handler.NotHandled
	}

	// Extract artist name from current selection
	artistName := m.extractArtistFromSelection()
	if artistName == "" {
		return handler.NotHandled
	}

	cmd := m.Popups.ShowSimilarArtists(m.Lastfm, m.Library, artistName)
	return handler.Handled(cmd)
}

// extractArtistFromSelection returns the artist name from the current navigator selection.
func (m *Model) extractArtistFromSelection() string {
	if m.Navigation.IsBrowserViewActive() {
		return m.Navigation.LibraryBrowser().SelectedArtist()
	}

	if m.Navigation.IsAlbumViewActive() {
		// Album view: get album artist from selected album
		if album := m.Navigation.AlbumView().SelectedAlbum(); album != nil {
			return album.AlbumArtist
		}
		return ""
	}

	// Miller view: check current level
	nav := m.Navigation.LibraryNav()
	selected := nav.Selected()
	if selected == nil {
		return ""
	}

	// Determine artist based on node type
	switch selected.Level() {
	case library.LevelRoot:
		// Root level - no artist
		return ""
	case library.LevelArtist:
		// Artist level - use the artist name directly
		return selected.Artist()
	case library.LevelAlbum:
		// Album level - get artist from the selected album
		return selected.Artist()
	case library.LevelTrack:
		// Track level - get album artist from track
		if track := selected.Track(); track != nil {
			return track.AlbumArtist
		}
	}
	return ""
}
