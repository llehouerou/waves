package playlistnav

import (
	"github.com/llehouerou/waves/internal/core"
)

// PlaylistInOrOn is the playlist a navigator is in, its current node, or on,
// its selected node; nil when it is in or on no playlist.
func PlaylistInOrOn(current Node, selected *Node) *int64 {
	if current.level == LevelPlaylist {
		return current.playlistID
	}
	if selected != nil && selected.level == LevelPlaylist {
		return selected.playlistID
	}
	return nil
}

// LocateTrack returns the ID of the node of the library track trackID at its
// first place in a playlist that holds it: preferred when it holds it (nil
// for none), else Favorites, else the first playlist the view lists, read
// from the root in the source's order, each folder with everything under it.
// It finds nothing, with no error, for a track in no playlist
// and for the ID 0; a read that fails is returned.
func LocateTrack(p Playlists, trackID int64, preferred *int64) (nodeID string, found bool, err error) {
	if trackID == 0 {
		return "", false, nil
	}
	src := NewSource(p)
	l := locator{source: src, trackID: trackID, read: map[int64]bool{}}
	if preferred != nil {
		if id, ok, err := l.in(*preferred); ok || err != nil {
			return id, ok, err
		}
	}
	if id, ok, err := l.in(core.FavoritesPlaylistID); ok || err != nil {
		return id, ok, err
	}
	return l.under(src.Root())
}

// locator looks for a track in the playlists, reading each one once.
type locator struct {
	source  *Source
	trackID int64
	read    map[int64]bool
}

// in looks for the track in the playlist id.
func (l *locator) in(id int64) (nodeID string, found bool, err error) {
	if l.read[id] {
		return "", false, nil
	}
	l.read[id] = true
	tracks, err := l.source.playlists.Tracks(id)
	if err != nil {
		return "", false, err
	}
	for i := range tracks {
		if tracks[i].ID == l.trackID {
			return TrackNodeID(id, i), true, nil
		}
	}
	return "", false, nil
}

// under looks for the track under a container, the root or a folder, in the
// order the view lists its children, a folder with everything under it.
func (l *locator) under(container Node) (nodeID string, found bool, err error) {
	children, err := l.source.Children(container)
	if err != nil {
		return "", false, err
	}
	for _, child := range children {
		switch child.level { //nolint:exhaustive // the root and the tracks are no child of a container
		case LevelFolder:
			nodeID, found, err = l.under(child)
		case LevelPlaylist:
			nodeID, found, err = l.in(*child.playlistID)
		}
		if found || err != nil {
			return nodeID, found, err
		}
	}
	return "", false, nil
}
