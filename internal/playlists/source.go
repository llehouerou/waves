package playlists

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/playlist"
)

const playlistsRootPath = "Playlists"

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
		return "playlists:root"
	case LevelFolder:
		if n.folderID != nil {
			return "playlists:folder:" + strconv.FormatInt(*n.folderID, 10)
		}
		return ""
	case LevelPlaylist:
		if n.playlistID != nil {
			return "playlists:playlist:" + strconv.FormatInt(*n.playlistID, 10)
		}
		return ""
	case LevelTrack:
		if n.playlistID != nil {
			return fmt.Sprintf("playlists:track:%d:%d", *n.playlistID, n.position)
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

// Source implements navigator.Source for playlist browsing.
type Source struct {
	playlists *Playlists
}

// NewSource creates a new playlist source.
func NewSource(p *Playlists) *Source {
	return &Source{playlists: p}
}

// Root returns the root node.
func (s *Source) Root() Node {
	return Node{
		level: LevelRoot,
		name:  playlistsRootPath,
	}
}

// Children returns the children of a node.
func (s *Source) Children(parent Node) ([]Node, error) {
	switch parent.level {
	case LevelRoot:
		return s.rootChildren()
	case LevelFolder:
		return s.folderChildren(parent.folderID)
	case LevelPlaylist:
		return s.playlistChildren(parent.playlistID)
	case LevelTrack:
		return nil, nil
	}
	return nil, nil
}

// rootChildren returns folders and playlists at the root level.
func (s *Source) rootChildren() ([]Node, error) {
	// Get root-level folders
	folders, err := s.playlists.Folders(nil)
	if err != nil {
		return nil, err
	}

	// Get root-level playlists
	pls, err := s.playlists.List(nil)
	if err != nil {
		return nil, err
	}

	nodes := make([]Node, 0, len(folders)+len(pls))

	for _, f := range folders {
		id := f.ID
		nodes = append(nodes, Node{
			level:    LevelFolder,
			folderID: &id,
			name:     f.Name,
		})
	}

	for _, pl := range pls {
		id := pl.ID
		nodes = append(nodes, Node{
			level:              LevelPlaylist,
			playlistID:         &id,
			name:               pl.Name,
			containingFolderID: nil, // Root level
		})
	}

	return nodes, nil
}

// folderChildren returns folders and playlists inside a folder.
func (s *Source) folderChildren(folderID *int64) ([]Node, error) {
	if folderID == nil {
		return s.rootChildren()
	}

	// Get sub-folders
	folders, err := s.playlists.Folders(folderID)
	if err != nil {
		return nil, err
	}

	// Get playlists in this folder
	pls, err := s.playlists.List(folderID)
	if err != nil {
		return nil, err
	}

	nodes := make([]Node, 0, len(folders)+len(pls))

	for _, f := range folders {
		id := f.ID
		nodes = append(nodes, Node{
			level:    LevelFolder,
			folderID: &id,
			name:     f.Name,
		})
	}

	for _, pl := range pls {
		id := pl.ID
		nodes = append(nodes, Node{
			level:              LevelPlaylist,
			playlistID:         &id,
			name:               pl.Name,
			containingFolderID: folderID,
		})
	}

	return nodes, nil
}

// playlistChildren returns tracks in a playlist.
func (s *Source) playlistChildren(playlistID *int64) ([]Node, error) {
	if playlistID == nil {
		return nil, nil
	}

	tracks, err := s.playlists.Tracks(*playlistID)
	if err != nil {
		return nil, err
	}

	// Get the playlist's containing folder for context
	var containingFolderID *int64
	if pl, err := s.playlists.Get(*playlistID); err == nil {
		containingFolderID = pl.FolderID
	}

	nodes := make([]Node, len(tracks))
	for i := range tracks {
		track := &tracks[i]
		name := track.Title
		if track.Artist != "" {
			name = track.Artist + " - " + name
		}
		if track.TrackNumber > 0 {
			name = fmt.Sprintf("%02d. %s", track.TrackNumber, name)
		}
		nodes[i] = Node{
			level:              LevelTrack,
			playlistID:         playlistID,
			position:           i,
			track:              track,
			name:               name,
			containingFolderID: containingFolderID,
		}
	}
	return nodes, nil
}

// Parent returns the parent of a node.
func (s *Source) Parent(node Node) *Node {
	switch node.level {
	case LevelRoot:
		return nil
	case LevelFolder:
		if node.folderID == nil {
			return nil
		}
		folder, err := s.playlists.FolderByID(*node.folderID)
		if err != nil {
			return nil
		}
		if folder.ParentID == nil {
			root := s.Root()
			return &root
		}
		parentFolder, err := s.playlists.FolderByID(*folder.ParentID)
		if err != nil {
			root := s.Root()
			return &root
		}
		return &Node{
			level:    LevelFolder,
			folderID: folder.ParentID,
			name:     parentFolder.Name,
		}
	case LevelPlaylist:
		if node.playlistID == nil {
			return nil
		}
		pl, err := s.playlists.Get(*node.playlistID)
		if err != nil {
			return nil
		}
		if pl.FolderID == nil {
			root := s.Root()
			return &root
		}
		folder, err := s.playlists.FolderByID(*pl.FolderID)
		if err != nil {
			root := s.Root()
			return &root
		}
		return &Node{
			level:    LevelFolder,
			folderID: pl.FolderID,
			name:     folder.Name,
		}
	case LevelTrack:
		if node.playlistID == nil {
			return nil
		}
		pl, err := s.playlists.Get(*node.playlistID)
		if err != nil {
			return nil
		}
		return &Node{
			level:      LevelPlaylist,
			playlistID: node.playlistID,
			name:       pl.Name,
		}
	}
	return nil
}

// DisplayPath returns a human-readable path for display.
// Uses icons to distinguish folders from playlists.
func (s *Source) DisplayPath(node Node) string {
	switch node.level {
	case LevelRoot:
		return icons.FormatDir(playlistsRootPath)
	case LevelFolder:
		if node.folderID == nil {
			return icons.FormatDir(playlistsRootPath)
		}
		path := s.buildFolderPathWithIcons(*node.folderID)
		return icons.FormatDir(playlistsRootPath) + " > " + path
	case LevelPlaylist:
		if node.playlistID == nil {
			return icons.FormatDir(playlistsRootPath)
		}
		pl, err := s.playlists.Get(*node.playlistID)
		if err != nil {
			return icons.FormatDir(playlistsRootPath)
		}
		playlistName := icons.FormatPlaylist(pl.Name)
		if pl.FolderID != nil {
			path := s.buildFolderPathWithIcons(*pl.FolderID)
			return icons.FormatDir(playlistsRootPath) + " > " + path + " > " + playlistName
		}
		return icons.FormatDir(playlistsRootPath) + " > " + playlistName
	case LevelTrack:
		if node.playlistID == nil {
			return icons.FormatDir(playlistsRootPath)
		}
		pl, err := s.playlists.Get(*node.playlistID)
		if err != nil {
			return icons.FormatDir(playlistsRootPath)
		}
		playlistName := icons.FormatPlaylist(pl.Name)
		if pl.FolderID != nil {
			path := s.buildFolderPathWithIcons(*pl.FolderID)
			return icons.FormatDir(playlistsRootPath) + " > " + path + " > " + playlistName
		}
		return icons.FormatDir(playlistsRootPath) + " > " + playlistName
	}
	return icons.FormatDir(playlistsRootPath)
}

// buildFolderPathWithIcons builds the path string for a folder with folder icons.
func (s *Source) buildFolderPathWithIcons(folderID int64) string {
	var parts []string
	currentID := folderID

	for {
		folder, err := s.playlists.FolderByID(currentID)
		if err != nil {
			break
		}
		parts = append([]string{icons.FormatDir(folder.Name)}, parts...)
		if folder.ParentID == nil {
			break
		}
		currentID = *folder.ParentID
	}

	return strings.Join(parts, " > ")
}

// NodeFromID creates a node from its ID.
func (s *Source) NodeFromID(id string) (Node, bool) {
	parts := strings.SplitN(id, ":", 4)
	if len(parts) < 2 || parts[0] != "playlists" {
		return Node{}, false
	}

	switch parts[1] {
	case "root":
		return s.Root(), true
	case "folder":
		if len(parts) < 3 {
			return Node{}, false
		}
		folderID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return Node{}, false
		}
		folder, err := s.playlists.FolderByID(folderID)
		if err != nil {
			return Node{}, false
		}
		return Node{
			level:    LevelFolder,
			folderID: &folderID,
			name:     folder.Name,
		}, true
	case "playlist":
		if len(parts) < 3 {
			return Node{}, false
		}
		playlistID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return Node{}, false
		}
		pl, err := s.playlists.Get(playlistID)
		if err != nil {
			return Node{}, false
		}
		return Node{
			level:      LevelPlaylist,
			playlistID: &playlistID,
			name:       pl.Name,
		}, true
	case "track":
		if len(parts) < 4 {
			return Node{}, false
		}
		playlistID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return Node{}, false
		}
		position, err := strconv.Atoi(parts[3])
		if err != nil {
			return Node{}, false
		}
		tracks, err := s.playlists.Tracks(playlistID)
		if err != nil || position >= len(tracks) {
			return Node{}, false
		}
		track := &tracks[position]
		name := track.Title
		if track.Artist != "" {
			name = track.Artist + " - " + name
		}
		if track.TrackNumber > 0 {
			name = fmt.Sprintf("%02d. %s", track.TrackNumber, name)
		}
		return Node{
			level:      LevelTrack,
			playlistID: &playlistID,
			position:   position,
			track:      track,
			name:       name,
		}, true
	}
	return Node{}, false
}
