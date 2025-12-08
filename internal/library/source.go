package library

import (
	"fmt"

	"github.com/llehouerou/waves/internal/navigator/sourceutil"
)

// Source implements navigator.Source for library browsing.
type Source struct {
	lib *Library
}

// NewSource creates a new library source.
func NewSource(lib *Library) *Source {
	return &Source{lib: lib}
}

// Root returns the root node.
func (s *Source) Root() Node {
	return Node{
		level: LevelRoot,
		name:  libraryRootPath,
	}
}

// Children returns the children of a node.
func (s *Source) Children(parent Node) ([]Node, error) {
	switch parent.level {
	case LevelRoot:
		return s.artistChildren()
	case LevelArtist:
		return s.albumChildren(parent.artist)
	case LevelAlbum:
		return s.trackChildren(parent.artist, parent.album)
	case LevelTrack:
		return nil, nil
	}
	return nil, nil
}

// artistChildren returns all artists.
func (s *Source) artistChildren() ([]Node, error) {
	artists, err := s.lib.Artists()
	if err != nil {
		return nil, err
	}
	nodes := make([]Node, len(artists))
	for i, artist := range artists {
		nodes[i] = Node{
			level:  LevelArtist,
			artist: artist,
			name:   artist,
		}
	}
	return nodes, nil
}

// albumChildren returns albums for an artist.
func (s *Source) albumChildren(artist string) ([]Node, error) {
	albums, err := s.lib.Albums(artist)
	if err != nil {
		return nil, err
	}
	nodes := make([]Node, len(albums))
	for i, album := range albums {
		name := album.Name
		if album.Year > 0 {
			name = fmt.Sprintf("[%d] %s", album.Year, album.Name)
		}
		nodes[i] = Node{
			level:     LevelAlbum,
			artist:    artist,
			album:     album.Name,
			albumYear: album.Year,
			name:      name,
		}
	}
	return nodes, nil
}

// trackChildren returns tracks for an album.
func (s *Source) trackChildren(artist, album string) ([]Node, error) {
	tracks, err := s.lib.Tracks(artist, album)
	if err != nil {
		return nil, err
	}

	// Check if album has multiple discs
	hasMultipleDiscs := false
	for i := range tracks {
		if tracks[i].DiscNumber > 1 {
			hasMultipleDiscs = true
			break
		}
	}

	nodes := make([]Node, len(tracks))
	for i := range tracks {
		track := &tracks[i]
		name := track.Title
		// Show track artist if different from album artist
		if track.Artist != "" && track.Artist != track.AlbumArtist {
			name = track.Artist + " - " + name
		}
		if track.TrackNumber > 0 {
			if hasMultipleDiscs && track.DiscNumber > 0 {
				// Format as "D.TT. Title" for multi-disc albums
				name = fmt.Sprintf("%d.%02d. %s", track.DiscNumber, track.TrackNumber, name)
			} else {
				name = fmt.Sprintf("%02d. %s", track.TrackNumber, name)
			}
		}
		nodes[i] = Node{
			level:  LevelTrack,
			artist: artist,
			album:  album,
			track:  track,
			name:   name,
		}
	}
	return nodes, nil
}

// Parent returns the parent of a node.
func (s *Source) Parent(node Node) *Node {
	switch node.level {
	case LevelRoot:
		return nil
	case LevelArtist:
		root := s.Root()
		return &root
	case LevelAlbum:
		parent := Node{
			level:  LevelArtist,
			artist: node.artist,
			name:   node.artist,
		}
		return &parent
	case LevelTrack:
		parent := Node{
			level:  LevelAlbum,
			artist: node.artist,
			album:  node.album,
			name:   node.album,
		}
		return &parent
	}
	return nil
}

// NodeFromID creates a node from its ID.
func (s *Source) NodeFromID(id string) (Node, bool) {
	parts, ok := sourceutil.ParseID(id, "library")
	if !ok {
		return Node{}, false
	}

	switch parts[0] {
	case "root":
		return s.Root(), true
	case "artist":
		if len(parts) < 2 {
			return Node{}, false
		}
		return Node{
			level:  LevelArtist,
			artist: parts[1],
			name:   parts[1],
		}, true
	case "album":
		if len(parts) < 3 {
			return Node{}, false
		}
		return Node{
			level:  LevelAlbum,
			artist: parts[1],
			album:  parts[2],
			name:   parts[2],
		}, true
	case "track":
		if len(parts) < 2 {
			return Node{}, false
		}
		trackID, ok := sourceutil.ParseInt64(parts[1])
		if !ok {
			return Node{}, false
		}
		track, err := s.lib.TrackByID(trackID)
		if err != nil {
			return Node{}, false
		}
		name := track.Title
		if track.Artist != "" && track.Artist != track.AlbumArtist {
			name = track.Artist + " - " + name
		}
		if track.TrackNumber > 0 {
			hasMultipleDiscs, _ := s.lib.AlbumHasMultipleDiscs(track.AlbumArtist, track.Album)
			if hasMultipleDiscs && track.DiscNumber > 0 {
				name = fmt.Sprintf("%d.%02d. %s", track.DiscNumber, track.TrackNumber, name)
			} else {
				name = fmt.Sprintf("%02d. %s", track.TrackNumber, name)
			}
		}
		return Node{
			level:  LevelTrack,
			artist: track.AlbumArtist,
			album:  track.Album,
			track:  track,
			name:   name,
		}, true
	}
	return Node{}, false
}
