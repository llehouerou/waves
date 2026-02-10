// internal/app/queue.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
)

// HandleQueueAction performs the specified queue action on the selected item.
func (m *Model) HandleQueueAction(action QueueAction) tea.Cmd {
	tracks, err := m.collectTracksFromSelected()
	if err != nil {
		m.Popups.ShowOpError(errmsg.OpQueueAdd, err)
		return nil
	}
	if len(tracks) == 0 {
		return nil
	}

	// Convert to playback tracks
	pbTracks := playback.TracksFromPlaylist(tracks)

	var trackToPlay *playback.Track

	switch action {
	case QueueAdd:
		m.PlaybackService.AddTracks(pbTracks...)
	case QueueReplace:
		trackToPlay = m.PlaybackService.ReplaceTracks(pbTracks...)
	}

	m.SaveQueueState()
	m.Layout.QueuePanel().SyncCursor()
	// Clear preloaded track since queue contents changed
	m.PlaybackService.Player().ClearPreload()

	if trackToPlay != nil {
		if err := m.PlaybackService.Play(); err != nil {
			m.Popups.ShowOpError(errmsg.OpPlaybackStart, err)
		}
	}
	return nil
}

// collectTracksFromSelected returns tracks from the currently selected item.
// Returns nil, nil if no item is selected.
func (m *Model) collectTracksFromSelected() ([]playlist.Track, error) {
	switch m.Navigation.ViewMode() {
	case navctl.ViewFileBrowser:
		if sel := m.Navigation.FileNav().Selected(); sel != nil {
			return playlist.CollectFromFileNode(*sel)
		}
	case navctl.ViewLibrary:
		if m.Navigation.IsBrowserViewActive() {
			return m.collectTracksFromBrowser()
		}
		if sel := m.Navigation.LibraryNav().Selected(); sel != nil {
			return playlist.CollectFromLibraryNode(m.Library, *sel)
		}
	case navctl.ViewPlaylists:
		if sel := m.Navigation.PlaylistNav().Selected(); sel != nil {
			return collectFromPlaylistNode(m.Playlists, *sel)
		}
	case navctl.ViewDownloads:
		// Downloads view doesn't support queue actions
		return nil, nil
	}
	return nil, nil
}

// collectFromPlaylistNode collects tracks from a playlist node.
func collectFromPlaylistNode(pls *playlists.Playlists, node playlists.Node) ([]playlist.Track, error) {
	switch node.Level() {
	case playlists.LevelRoot, playlists.LevelFolder:
		// Can't play folders
		return nil, nil
	case playlists.LevelPlaylist:
		playlistID := node.PlaylistID()
		if playlistID == nil {
			return nil, nil
		}
		return pls.Tracks(*playlistID)
	case playlists.LevelTrack:
		if t := node.Track(); t != nil {
			return []playlist.Track{*t}, nil
		}
		return nil, nil
	default:
		return nil, nil
	}
}

// HandleContainerAndPlay replaces queue with all tracks in the container
// (album for library, playlist for playlists, folder for file browser)
// and plays from the selected track.
func (m *Model) HandleContainerAndPlay() tea.Cmd {
	var tracks []playlist.Track
	var selectedIdx int
	var err error

	switch m.Navigation.ViewMode() {
	case navctl.ViewLibrary:
		if m.Navigation.IsBrowserViewActive() {
			tracks, selectedIdx, err = m.collectAlbumFromBrowserTrack()
		} else {
			selected := m.Navigation.LibraryNav().Selected()
			if selected == nil {
				return nil
			}
			tracks, selectedIdx, err = playlist.CollectAlbumFromTrack(m.Library, *selected)
		}
	case navctl.ViewPlaylists:
		selected := m.Navigation.PlaylistNav().Selected()
		if selected == nil {
			return nil
		}
		tracks, selectedIdx, err = m.collectPlaylistFromNode(*selected)
	case navctl.ViewFileBrowser:
		selected := m.Navigation.FileNav().Selected()
		if selected == nil {
			return nil
		}
		tracks, selectedIdx, err = playlist.CollectFolderFromFile(*selected)
	case navctl.ViewDownloads:
		// Not supported for downloads view
		return nil
	}

	if err != nil {
		m.Popups.ShowOpError(errmsg.OpQueueAdd, err)
		return nil
	}

	if len(tracks) == 0 {
		return nil
	}

	// Convert to playback tracks
	pbTracks := playback.TracksFromPlaylist(tracks)
	m.PlaybackService.ReplaceTracks(pbTracks...)
	trackToPlay := m.PlaybackService.QueueMoveTo(selectedIdx)

	m.SaveQueueState()
	m.Layout.QueuePanel().SyncCursor()
	// Clear preloaded track since queue was replaced
	m.PlaybackService.Player().ClearPreload()

	if trackToPlay != nil {
		if err := m.PlaybackService.Play(); err != nil {
			m.Popups.ShowOpError(errmsg.OpPlaybackStart, err)
		}
	}
	return nil
}

// collectPlaylistFromNode collects all tracks from a playlist given a node.
// If the node is a track, it returns the full playlist with the track's position.
// If the node is a playlist, it returns all tracks starting from position 0.
func (m *Model) collectPlaylistFromNode(node playlists.Node) ([]playlist.Track, int, error) {
	switch node.Level() {
	case playlists.LevelTrack:
		playlistID := node.PlaylistID()
		if playlistID == nil {
			return nil, 0, nil
		}
		return playlist.CollectPlaylistFromTrack(m.Playlists, *playlistID, node.Position())
	case playlists.LevelPlaylist:
		playlistID := node.PlaylistID()
		if playlistID == nil {
			return nil, 0, nil
		}
		tracks, err := m.Playlists.Tracks(*playlistID)
		return tracks, 0, err
	case playlists.LevelRoot, playlists.LevelFolder:
		// Can't play root or folders directly
		return nil, 0, nil
	}
	return nil, 0, nil
}

// collectTracksFromBrowser returns tracks from the browser's current selection.
func (m *Model) collectTracksFromBrowser() ([]playlist.Track, error) {
	browser := m.Navigation.LibraryBrowser()
	artist := browser.SelectedArtist()

	switch browser.ActiveColumn() {
	case librarybrowser.ColumnArtists:
		// All tracks for this artist
		if artist == "" {
			return nil, nil
		}
		albums, err := m.Library.Albums(artist)
		if err != nil {
			return nil, err
		}
		var tracks []playlist.Track
		for _, album := range albums {
			albumTracks, err := collectAlbumTracks(m.Library, artist, album.Name)
			if err != nil {
				continue
			}
			tracks = append(tracks, albumTracks...)
		}
		return tracks, nil
	case librarybrowser.ColumnAlbums:
		album := browser.SelectedAlbum()
		if album == nil {
			return nil, nil
		}
		return collectAlbumTracks(m.Library, artist, album.Name)
	case librarybrowser.ColumnTracks:
		track := browser.SelectedTrack()
		if track == nil {
			return nil, nil
		}
		return []playlist.Track{playlist.FromLibraryTrack(*track)}, nil
	}
	return nil, nil
}

// collectAlbumFromBrowserTrack collects all album tracks and returns the selected track index.
func (m *Model) collectAlbumFromBrowserTrack() ([]playlist.Track, int, error) {
	browser := m.Navigation.LibraryBrowser()
	artist := browser.SelectedArtist()
	album := browser.SelectedAlbum()

	if artist == "" || album == nil {
		return nil, 0, nil
	}

	albumTracks, err := m.Library.Tracks(artist, album.Name)
	if err != nil {
		return nil, 0, err
	}

	// Find the index of the selected track
	selectedIdx := 0
	if track := browser.SelectedTrack(); track != nil {
		for i := range albumTracks {
			if albumTracks[i].ID == track.ID {
				selectedIdx = i
				break
			}
		}
	}

	return playlist.FromLibraryTracks(albumTracks), selectedIdx, nil
}

// collectAlbumTracks collects all tracks for an album as playlist tracks.
func collectAlbumTracks(lib *library.Library, artist, album string) ([]playlist.Track, error) {
	trackIDs, err := lib.AlbumTrackIDs(artist, album)
	if err != nil {
		return nil, err
	}
	tracks := make([]playlist.Track, 0, len(trackIDs))
	for _, id := range trackIDs {
		t, err := lib.TrackByID(id)
		if err != nil {
			continue
		}
		tracks = append(tracks, playlist.FromLibraryTrack(*t))
	}
	return tracks, nil
}
