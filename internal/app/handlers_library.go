// internal/app/handlers_library.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/ui/albumview"
)

// handleLibraryKeys handles library-specific keys (d for delete, f for favorite, V for album view).
func (m *Model) handleLibraryKeys(key string) (bool, tea.Cmd) {
	if m.Navigation.ViewMode() != ViewLibrary || !m.Navigation.IsNavigatorFocused() {
		return false, nil
	}

	// V toggles album view sub-mode (works regardless of selection)
	if key == "V" {
		m.toggleLibraryViewMode()
		return true, nil
	}

	// Album view doesn't use these keys - they're handled by the view itself
	if m.Navigation.LibrarySubMode() == LibraryModeAlbum {
		return false, nil
	}

	selected := m.Navigation.LibraryNav().Selected()
	if selected == nil {
		return false, nil
	}

	switch key {
	case "F":
		// Toggle favorite - only at track level
		if selected.Level() != library.LevelTrack {
			return false, nil
		}
		track := selected.Track()
		if track == nil {
			return true, nil
		}
		return m.handleToggleFavorite([]int64{track.ID})

	case "d":
		// Delete track - only at track level
		if selected.Level() != library.LevelTrack {
			return false, nil
		}

		track := selected.Track()
		if track == nil {
			return true, nil
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
		return true, nil
	}

	return false, nil
}

// handleToggleFavorite toggles favorite status for the given track IDs.
func (m *Model) handleToggleFavorite(trackIDs []int64) (bool, tea.Cmd) {
	if len(trackIDs) == 0 {
		return true, nil
	}

	results, err := m.Playlists.ToggleFavorites(trackIDs)
	if err != nil {
		m.Popups.ShowError(errmsg.Format(errmsg.OpFavoriteToggle, err))
		return true, nil
	}

	// Refresh favorites in navigators
	m.RefreshFavorites()

	// Refresh playlist navigator (Favorites playlist contents changed)
	// This ensures the Favorites playlist shows correct tracks when viewed
	m.refreshPlaylistNavigatorInPlace()

	_ = results // results used for refreshing, no message needed
	return true, nil
}

// handleQueueAlbum handles the QueueAlbumMsg from the album view.
func (m Model) handleQueueAlbum(msg albumview.QueueAlbumMsg) (tea.Model, tea.Cmd) {
	trackIDs, err := m.Library.AlbumTrackIDs(msg.AlbumArtist, msg.Album)
	if err != nil || len(trackIDs) == 0 {
		return m, nil
	}

	tracks := make([]playlist.Track, 0, len(trackIDs))
	for _, id := range trackIDs {
		t, err := m.Library.TrackByID(id)
		if err != nil {
			continue
		}
		tracks = append(tracks, playlist.Track{
			ID:          t.ID,
			Path:        t.Path,
			Title:       t.Title,
			Artist:      t.Artist,
			Album:       t.Album,
			TrackNumber: t.TrackNumber,
		})
	}

	if len(tracks) == 0 {
		return m, nil
	}

	if msg.Replace {
		m.Playback.Queue().Clear()
	}
	m.Playback.Queue().Add(tracks...)
	m.SaveQueueState()

	if msg.Replace {
		cmd := m.PlayTrackAtIndex(0)
		return m, cmd
	}

	return m, nil
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
	if m.Navigation.LibrarySubMode() == LibraryModeAlbum {
		if a := m.Navigation.AlbumView().SelectedAlbum(); a != nil {
			return a.AlbumArtist, a.Album
		}
	} else {
		if selected := m.Navigation.LibraryNav().Selected(); selected != nil {
			return selected.Artist(), selected.Album()
		}
	}
	return "", ""
}

// selectAlbumInCurrentMode selects the album in the current library sub-mode.
func (m *Model) selectAlbumInCurrentMode(albumArtist, albumName string) {
	if m.Navigation.LibrarySubMode() == LibraryModeAlbum {
		if err := m.Navigation.AlbumView().Refresh(); err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpAlbumLoad, err))
			return
		}
		if albumArtist != "" && albumName != "" {
			m.Navigation.AlbumView().SelectByID(albumArtist + ":" + albumName)
		}
	} else if albumArtist != "" && albumName != "" {
		albumID := "library:album:" + albumArtist + ":" + albumName
		m.Navigation.LibraryNav().NavigateTo(albumID)
	}
}
