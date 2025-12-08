package playlists

import (
	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/navigator/sourceutil"
	"github.com/llehouerou/waves/internal/playlist"
)

// Level represents the hierarchy level of a playlist node.
type Level int

const (
	LevelRoot Level = iota
	LevelFolder
	LevelPlaylist
	LevelTrack
)

// Node represents a node in the playlist hierarchy.
type Node struct {
	level              Level
	folderID           *int64
	playlistID         *int64
	position           int // Position in playlist (for tracks)
	track              *playlist.Track
	name               string
	containingFolderID *int64 // Folder containing this playlist/track
}

// ID returns a unique identifier for this node.
func (n Node) ID() string {
	switch n.level {
	case LevelRoot:
		return sourceutil.FormatID("playlists", "root")
	case LevelFolder:
		if n.folderID != nil {
			return sourceutil.FormatID("playlists", "folder", sourceutil.FormatInt64(*n.folderID))
		}
		return ""
	case LevelPlaylist:
		if n.playlistID != nil {
			return sourceutil.FormatID("playlists", "playlist", sourceutil.FormatInt64(*n.playlistID))
		}
		return ""
	case LevelTrack:
		if n.playlistID != nil {
			return sourceutil.FormatID("playlists", "track", sourceutil.FormatInt64(*n.playlistID), sourceutil.FormatInt(n.position))
		}
		return ""
	}
	return ""
}

// DisplayName returns the display text for this node.
func (n Node) DisplayName() string {
	return n.name
}

// IsContainer returns true if this node can be navigated into.
func (n Node) IsContainer() bool {
	return n.level != LevelTrack
}

// IconType returns the icon type for this node.
func (n Node) IconType() navigator.IconType {
	switch n.level {
	case LevelRoot, LevelFolder:
		return navigator.IconFolder
	case LevelPlaylist:
		return navigator.IconPlaylist
	case LevelTrack:
		return navigator.IconAudio
	default:
		return navigator.IconFolder
	}
}

// Level returns the hierarchy level of this node.
func (n Node) Level() Level {
	return n.level
}

// FolderID returns the folder ID for folder nodes.
func (n Node) FolderID() *int64 {
	return n.folderID
}

// PlaylistID returns the playlist ID for playlist or track nodes.
func (n Node) PlaylistID() *int64 {
	return n.playlistID
}

// Position returns the position in the playlist for track nodes.
func (n Node) Position() int {
	return n.position
}

// Track returns the track data for track nodes, nil otherwise.
func (n Node) Track() *playlist.Track {
	return n.track
}

// ParentFolderID returns the folder ID that contains this node.
// For folders, this returns the folder's own ID (to create inside it).
// For playlists/tracks, this returns the containing folder's ID.
func (n Node) ParentFolderID() *int64 {
	switch n.level {
	case LevelRoot:
		return nil
	case LevelFolder:
		return n.folderID
	case LevelPlaylist, LevelTrack:
		return n.containingFolderID
	}
	return nil
}

// NodeItem wraps a Node for local search (current level only).
type NodeItem struct {
	Node Node
}

// FilterValue returns the searchable text for filtering.
func (n NodeItem) FilterValue() string {
	return n.Node.DisplayName()
}

// DisplayText returns the display text for search results.
func (n NodeItem) DisplayText() string {
	switch n.Node.level {
	case LevelRoot:
		return n.Node.DisplayName()
	case LevelFolder:
		return icons.FormatDir(n.Node.DisplayName())
	case LevelPlaylist:
		return icons.FormatPlaylist(n.Node.DisplayName())
	case LevelTrack:
		return icons.FormatAudio(n.Node.DisplayName())
	}
	return n.Node.DisplayName()
}
