package playlistnav

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/navigator/sourceutil"
)

const playlistsRootPath = "Playlists"

// Playlists is what the source reads of the playlists.
type Playlists interface {
	Folders(parentID *int64) ([]core.PlaylistFolder, error)
	FolderByID(id int64) (*core.PlaylistFolder, error)
	List(folderID *int64) ([]core.Playlist, error)
	Get(id int64) (*core.Playlist, error)
	Tracks(id int64) ([]core.Track, error)
}

// Source implements navigator.Source for playlist browsing.
type Source struct {
	playlists Playlists
}

// NewSource creates a new playlist source.
func NewSource(p Playlists) *Source {
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

// NodeFromID creates a node from its ID. It finds no node, with no error, for
// an ID it cannot parse, a folder or a playlist that is not there, and a
// track position outside its playlist; a read that fails is returned.
func (s *Source) NodeFromID(id string) (Node, bool, error) {
	parts, ok := sourceutil.ParseID(id, "playlists")
	if !ok {
		return Node{}, false, nil
	}

	switch parts[0] {
	case "root":
		return s.Root(), true, nil
	case "folder":
		if len(parts) < 2 {
			return Node{}, false, nil
		}
		folderID, ok := sourceutil.ParseInt64(parts[1])
		if !ok {
			return Node{}, false, nil
		}
		folder, err := s.playlists.FolderByID(folderID)
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, false, nil
		}
		if err != nil {
			return Node{}, false, err
		}
		return Node{
			level:    LevelFolder,
			folderID: &folderID,
			name:     folder.Name,
		}, true, nil
	case "playlist":
		if len(parts) < 2 {
			return Node{}, false, nil
		}
		playlistID, ok := sourceutil.ParseInt64(parts[1])
		if !ok {
			return Node{}, false, nil
		}
		pl, err := s.playlists.Get(playlistID)
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, false, nil
		}
		if err != nil {
			return Node{}, false, err
		}
		return Node{
			level:      LevelPlaylist,
			playlistID: &playlistID,
			name:       pl.Name,
		}, true, nil
	case "track":
		if len(parts) < 3 {
			return Node{}, false, nil
		}
		playlistID, ok := sourceutil.ParseInt64(parts[1])
		if !ok {
			return Node{}, false, nil
		}
		position, ok := sourceutil.ParseInt(parts[2])
		if !ok {
			return Node{}, false, nil
		}
		tracks, err := s.playlists.Tracks(playlistID)
		if err != nil {
			return Node{}, false, err
		}
		if position < 0 || position >= len(tracks) {
			return Node{}, false, nil
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
		}, true, nil
	}
	return Node{}, false, nil
}
