// internal/app/queue.go
package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/ui/librarybrowser"
	"github.com/llehouerou/waves/internal/ui/librarynav"
	"github.com/llehouerou/waves/internal/ui/playlistnav"
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

	var trackToPlay *core.Track

	switch action {
	case QueueAdd:
		m.Core.Playback().AddTracks(tracks...)
	case QueueReplace:
		trackToPlay = m.Core.Playback().ReplaceTracks(tracks...)
	}

	m.Layout.QueuePanel().SyncCursor()

	if trackToPlay != nil {
		if err := m.Core.Playback().Play(); err != nil {
			m.Popups.ShowOpError(errmsg.OpPlaybackStart, err)
		}
	}
	return nil
}

// collectTracksFromSelected returns tracks from the currently selected item.
// Returns nil, nil if no item is selected.
func (m *Model) collectTracksFromSelected() ([]core.Track, error) {
	switch m.Navigation.ViewMode() {
	case navctl.ViewFileBrowser:
		if sel := m.Navigation.FileNav().Selected(); sel != nil {
			return core.TracksUnder(sel.ID())
		}
	case navctl.ViewLibrary:
		if m.Navigation.IsBrowserViewActive() {
			return m.collectTracksFromBrowser()
		}
		if sel := m.Navigation.LibraryNav().Selected(); sel != nil {
			return m.collectFromLibraryNode(*sel)
		}
	case navctl.ViewPlaylists:
		if sel := m.Navigation.PlaylistNav().Selected(); sel != nil {
			return m.collectFromPlaylistNode(*sel)
		}
	case navctl.ViewDownloads:
		// Downloads view doesn't support queue actions
		return nil, nil
	}
	return nil, nil
}

// collectFromLibraryNode collects all tracks for a library node.
// For artists: all tracks across all albums (sorted by album year, track number)
// For albums: all tracks (sorted by track number)
// For tracks: just that track
func (m *Model) collectFromLibraryNode(node librarynav.Node) ([]core.Track, error) {
	switch node.Level() {
	case librarynav.LevelRoot:
		// Root level - no tracks to collect
		return nil, nil
	case librarynav.LevelArtist:
		tracks, err := m.Library.ArtistTracks(node.Artist())
		return core.TracksFromLibrary(tracks), err
	case librarynav.LevelAlbum:
		tracks, err := m.Library.Tracks(node.Artist(), node.Album())
		return core.TracksFromLibrary(tracks), err
	case librarynav.LevelTrack:
		if t := node.Track(); t != nil {
			return core.TracksFromLibrary([]core.LibraryTrack{*t}), nil
		}
	}
	return nil, nil
}

// collectFromPlaylistNode collects tracks from a playlist node.
func (m *Model) collectFromPlaylistNode(node playlistnav.Node) ([]core.Track, error) {
	switch node.Level() {
	case playlistnav.LevelRoot, playlistnav.LevelFolder:
		// Can't play folders
		return nil, nil
	case playlistnav.LevelPlaylist:
		playlistID := node.PlaylistID()
		if playlistID == nil {
			return nil, nil
		}
		return m.Core.Playlists().Tracks(*playlistID)
	case playlistnav.LevelTrack:
		if t := node.Track(); t != nil {
			return []core.Track{*t}, nil
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
	var tracks []core.Track
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
			tracks, selectedIdx, err = m.collectAlbumFromLibraryNode(*selected)
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
		tracks, selectedIdx, err = core.FolderTracks(selected.ID())
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

	m.Core.Playback().ReplaceTracks(tracks...)
	trackToPlay := m.Core.Playback().QueueMoveTo(selectedIdx)

	m.Layout.QueuePanel().SyncCursor()

	if trackToPlay != nil {
		if err := m.Core.Playback().Play(); err != nil {
			m.Popups.ShowOpError(errmsg.OpPlaybackStart, err)
		}
	}
	return nil
}

// collectAlbumFromLibraryNode collects all tracks from the album containing
// the given track node, and the index of that track within the album. Any
// other node gives its own tracks, from the first.
func (m *Model) collectAlbumFromLibraryNode(node librarynav.Node) (tracks []core.Track, index int, err error) {
	if node.Level() != librarynav.LevelTrack {
		tracks, err = m.collectFromLibraryNode(node)
		return tracks, 0, err
	}
	track := node.Track()
	if track == nil {
		return nil, 0, nil
	}
	return m.collectAlbumAround(node.Artist(), node.Album(), track.ID)
}

// collectAlbumAround collects all tracks of an album and the index of the
// track with the given ID among them (0 when it is not there).
func (m *Model) collectAlbumAround(artist, album string, trackID int64) (tracks []core.Track, index int, err error) {
	albumTracks, err := m.Library.Tracks(artist, album)
	if err != nil {
		return nil, 0, err
	}

	for i := range albumTracks {
		if albumTracks[i].ID == trackID {
			index = i
			break
		}
	}

	return core.TracksFromLibrary(albumTracks), index, nil
}

// collectPlaylistFromNode collects all tracks from a playlist given a node.
// If the node is a track, it returns the full playlist with the track's position.
// If the node is a playlist, it returns all tracks starting from position 0.
func (m *Model) collectPlaylistFromNode(node playlistnav.Node) (tracks []core.Track, index int, err error) {
	playlistID := node.PlaylistID()
	if playlistID == nil {
		// Root and folders can't be played directly
		return nil, 0, nil
	}
	tracks, err = m.Core.Playlists().Tracks(*playlistID)
	if err != nil {
		return nil, 0, err
	}
	if node.Level() == playlistnav.LevelTrack && node.Position() < len(tracks) {
		index = node.Position()
	}
	return tracks, index, nil
}

// collectTracksFromBrowser returns tracks from the browser's current selection.
func (m *Model) collectTracksFromBrowser() ([]core.Track, error) {
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
		var tracks []core.Track
		for _, album := range albums {
			albumTracks, err := m.collectAlbumTracks(artist, album.Name)
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
		return m.collectAlbumTracks(artist, album.Name)
	case librarybrowser.ColumnTracks:
		track := browser.SelectedTrack()
		if track == nil {
			return nil, nil
		}
		return core.TracksFromLibrary([]core.LibraryTrack{*track}), nil
	}
	return nil, nil
}

// collectAlbumFromBrowserTrack collects all album tracks and returns the selected track index.
func (m *Model) collectAlbumFromBrowserTrack() (tracks []core.Track, index int, err error) {
	browser := m.Navigation.LibraryBrowser()
	artist := browser.SelectedArtist()
	album := browser.SelectedAlbum()

	if artist == "" || album == nil {
		return nil, 0, nil
	}

	var trackID int64
	if track := browser.SelectedTrack(); track != nil {
		trackID = track.ID
	}
	return m.collectAlbumAround(artist, album.Name, trackID)
}

// collectAlbumTracks collects all tracks for an album, in playback order.
func (m *Model) collectAlbumTracks(artist, album string) ([]core.Track, error) {
	trackIDs, err := m.Library.AlbumTrackIDs(artist, album)
	if err != nil {
		return nil, err
	}
	tracks := make([]core.LibraryTrack, 0, len(trackIDs))
	for _, id := range trackIDs {
		t, err := m.Library.TrackByID(id)
		if err != nil {
			continue
		}
		tracks = append(tracks, *t)
	}
	return core.TracksFromLibrary(tracks), nil
}
