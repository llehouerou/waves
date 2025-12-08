package playlists

import (
	"fmt"

	"github.com/llehouerou/waves/internal/navigator/sourceutil"
)

const playlistsRootPath = "Playlists"

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

// NodeFromID creates a node from its ID.
func (s *Source) NodeFromID(id string) (Node, bool) {
	parts, ok := sourceutil.ParseID(id, "playlists")
	if !ok {
		return Node{}, false
	}

	switch parts[0] {
	case "root":
		return s.Root(), true
	case "folder":
		if len(parts) < 2 {
			return Node{}, false
		}
		folderID, ok := sourceutil.ParseInt64(parts[1])
		if !ok {
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
		if len(parts) < 2 {
			return Node{}, false
		}
		playlistID, ok := sourceutil.ParseInt64(parts[1])
		if !ok {
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
		if len(parts) < 3 {
			return Node{}, false
		}
		playlistID, ok := sourceutil.ParseInt64(parts[1])
		if !ok {
			return Node{}, false
		}
		position, ok := sourceutil.ParseInt(parts[2])
		if !ok {
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
