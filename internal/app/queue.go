// internal/app/queue.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
)

// HandleQueueAction performs the specified queue action on the selected item.
func (m *Model) HandleQueueAction(action QueueAction) tea.Cmd {
	tracks, err := m.collectTracksFromSelected()
	if err != nil {
		m.Popups.ShowError(errmsg.Format(errmsg.OpQueueAdd, err))
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

	if trackToPlay != nil {
		if err := m.PlaybackService.Play(); err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPlaybackStart, err))
		}
	}
	return nil
}

// collectTracksFromSelected returns tracks from the currently selected item.
// Returns nil, nil if no item is selected.
func (m *Model) collectTracksFromSelected() ([]playlist.Track, error) {
	switch m.Navigation.ViewMode() {
	case ViewFileBrowser:
		if sel := m.Navigation.FileNav().Selected(); sel != nil {
			return playlist.CollectFromFileNode(*sel)
		}
	case ViewLibrary:
		if sel := m.Navigation.LibraryNav().Selected(); sel != nil {
			return playlist.CollectFromLibraryNode(m.Library, *sel)
		}
	case ViewPlaylists:
		if sel := m.Navigation.PlaylistNav().Selected(); sel != nil {
			return collectFromPlaylistNode(m.Playlists, *sel)
		}
	case ViewDownloads:
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
	case ViewLibrary:
		selected := m.Navigation.LibraryNav().Selected()
		if selected == nil {
			return nil
		}
		tracks, selectedIdx, err = playlist.CollectAlbumFromTrack(m.Library, *selected)
	case ViewPlaylists:
		selected := m.Navigation.PlaylistNav().Selected()
		if selected == nil {
			return nil
		}
		tracks, selectedIdx, err = m.collectPlaylistFromNode(*selected)
	case ViewFileBrowser:
		selected := m.Navigation.FileNav().Selected()
		if selected == nil {
			return nil
		}
		tracks, selectedIdx, err = playlist.CollectFolderFromFile(*selected)
	case ViewDownloads:
		// Not supported for downloads view
		return nil
	}

	if err != nil {
		m.Popups.ShowError(errmsg.Format(errmsg.OpQueueAdd, err))
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

	if trackToPlay != nil {
		if err := m.PlaybackService.Play(); err != nil {
			m.Popups.ShowError(errmsg.Format(errmsg.OpPlaybackStart, err))
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
