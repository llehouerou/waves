// internal/app/queue.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/playlist"
	"github.com/llehouerou/waves/internal/playlists"
)

// HandleQueueAction performs the specified queue action on the selected item.
func (m *Model) HandleQueueAction(action QueueAction) tea.Cmd {
	tracks, err := m.collectTracksFromSelected()
	if err != nil {
		m.Popups.ShowError(err.Error())
		return nil
	}
	if len(tracks) == 0 {
		return nil
	}

	var trackToPlay *playlist.Track

	switch action {
	case QueueAddAndPlay:
		trackToPlay = m.Queue.AddAndPlay(tracks...)
	case QueueAdd:
		m.Queue.Add(tracks...)
	case QueueReplace:
		trackToPlay = m.Queue.Replace(tracks...)
	}

	m.SaveQueueState()
	m.Layout.QueuePanel().SyncCursor()

	if trackToPlay != nil {
		return m.PlayTrack(trackToPlay.Path)
	}

	return nil
}

// collectTracksFromSelected returns tracks from the currently selected item.
// Returns nil, nil if no item is selected.
func (m *Model) collectTracksFromSelected() ([]playlist.Track, error) {
	switch m.ViewMode {
	case ViewFileBrowser:
		if sel := m.FileNavigator.Selected(); sel != nil {
			return playlist.CollectFromFileNode(*sel)
		}
	case ViewLibrary:
		if sel := m.LibraryNavigator.Selected(); sel != nil {
			return playlist.CollectFromLibraryNode(m.Library, *sel)
		}
	case ViewPlaylists:
		if sel := m.PlaylistNavigator.Selected(); sel != nil {
			return collectFromPlaylistNode(m.Playlists, *sel)
		}
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
// (album for library, playlist for playlists) and plays from the selected track.
func (m *Model) HandleContainerAndPlay() tea.Cmd {
	var tracks []playlist.Track
	var selectedIdx int
	var err error

	switch m.ViewMode {
	case ViewLibrary:
		selected := m.LibraryNavigator.Selected()
		if selected == nil {
			return nil
		}
		tracks, selectedIdx, err = playlist.CollectAlbumFromTrack(m.Library, *selected)
	case ViewPlaylists:
		selected := m.PlaylistNavigator.Selected()
		if selected == nil {
			return nil
		}
		tracks, selectedIdx, err = m.collectPlaylistFromNode(*selected)
	case ViewFileBrowser:
		// Not supported for file browser
		return nil
	}

	if err != nil {
		m.Popups.ShowError(err.Error())
		return nil
	}

	if len(tracks) == 0 {
		return nil
	}

	m.Queue.Replace(tracks...)
	trackToPlay := m.Queue.JumpTo(selectedIdx)

	m.SaveQueueState()
	m.Layout.QueuePanel().SyncCursor()

	if trackToPlay != nil {
		return m.PlayTrack(trackToPlay.Path)
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
