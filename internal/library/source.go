package library

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/navigator"
)

// SearchItem represents a library item for search results (deep search).
type SearchItem struct {
	Result SearchResult
}

func (s SearchItem) FilterValue() string {
	switch s.Result.Type {
	case ResultArtist:
		return s.Result.Artist
	case ResultAlbum:
		return s.Result.Album
	case ResultTrack:
		return s.Result.TrackTitle
	}
	return ""
}

func (s SearchItem) DisplayText() string {
	switch s.Result.Type {
	case ResultArtist:
		return icons.FormatArtist(s.Result.Artist)
	case ResultAlbum:
		display := s.Result.Album
		if s.Result.AlbumYear > 0 {
			display = fmt.Sprintf("[%d] %s", s.Result.AlbumYear, display)
		}
		return icons.FormatAlbum(s.Result.Artist + " - " + display)
	case ResultTrack:
		return icons.FormatAudio(s.Result.TrackArtist + " - " + s.Result.TrackTitle)
	}
	return ""
}

// NodeItem wraps a Node for local search (current level only).
type NodeItem struct {
	Node Node
}

func (n NodeItem) FilterValue() string {
	return n.Node.DisplayName()
}

func (n NodeItem) DisplayText() string {
	switch n.Node.level {
	case LevelRoot:
		return n.Node.DisplayName()
	case LevelArtist:
		return icons.FormatArtist(n.Node.DisplayName())
	case LevelAlbum:
		return icons.FormatAlbum(n.Node.DisplayName())
	case LevelTrack:
		return icons.FormatAudio(n.Node.DisplayName())
	}
	return n.Node.DisplayName()
}

type Level int

const (
	LevelRoot Level = iota
	LevelArtist
	LevelAlbum
	LevelTrack
)

// Node represents a node in the library hierarchy.
type Node struct {
	level     Level
	artist    string
	album     string
	albumYear int
	track     *Track
	name      string
}

func (n Node) ID() string {
	switch n.level {
	case LevelRoot:
		return "library:root"
	case LevelArtist:
		return "library:artist:" + n.artist
	case LevelAlbum:
		return "library:album:" + n.artist + ":" + n.album
	case LevelTrack:
		if n.track != nil {
			return "library:track:" + strconv.FormatInt(n.track.ID, 10)
		}
		return ""
	}
	return ""
}

func (n Node) DisplayName() string {
	return n.name
}

func (n Node) IsContainer() bool {
	return n.level != LevelTrack
}

func (n Node) IconType() navigator.IconType {
	switch n.level {
	case LevelRoot:
		return navigator.IconFolder
	case LevelArtist:
		return navigator.IconArtist
	case LevelAlbum:
		return navigator.IconAlbum
	case LevelTrack:
		return navigator.IconAudio
	default:
		return navigator.IconFolder
	}
}

// Path returns the file path for track nodes.
func (n Node) Path() string {
	if n.track != nil {
		return n.track.Path
	}
	return ""
}

// Level returns the hierarchy level of this node.
func (n Node) Level() Level {
	return n.level
}

// Artist returns the album artist for this node.
func (n Node) Artist() string {
	return n.artist
}

// Album returns the album name for this node.
func (n Node) Album() string {
	return n.album
}

// Track returns the track data for track nodes, nil otherwise.
func (n Node) Track() *Track {
	return n.track
}

// PreviewLines returns track metadata for display in the preview column.
// Implements navigator.PreviewProvider.
func (n Node) PreviewLines() []string {
	if n.level != LevelTrack || n.track == nil {
		return nil
	}

	t := n.track
	lines := []string{
		"",
		"  Title: " + t.Title,
		"  Artist: " + t.Artist,
	}

	if t.AlbumArtist != "" && t.AlbumArtist != t.Artist {
		lines = append(lines, "  Album Artist: "+t.AlbumArtist)
	}

	lines = append(lines, "  Album: "+t.Album)

	if t.Year > 0 {
		lines = append(lines, "  Year: "+strconv.Itoa(t.Year))
	}

	if t.TrackNumber > 0 {
		trackNum := strconv.Itoa(t.TrackNumber)
		if t.DiscNumber > 0 {
			trackNum = strconv.Itoa(t.DiscNumber) + "-" + trackNum
		}
		lines = append(lines, "  Track: "+trackNum)
	}

	if t.Genre != "" {
		lines = append(lines, "  Genre: "+t.Genre)
	}

	// Add path with wrapping to show full path
	lines = append(lines, "", "  Path:")
	lines = append(lines, wrapPath(t.Path, 40)...)

	return lines
}

// wrapPath wraps a path string into multiple lines with indent.
// Uses rune-based operations to handle Unicode characters correctly.
func wrapPath(path string, maxWidth int) []string {
	const indent = "  "
	contentWidth := maxWidth - len(indent)
	if contentWidth <= 0 {
		contentWidth = 20
	}

	runes := []rune(path)
	var lines []string

	for len(runes) > 0 {
		if len(runes) <= contentWidth {
			lines = append(lines, indent+string(runes))
			break
		}
		// Find a good break point (prefer after /)
		breakAt := contentWidth
		for i := contentWidth; i > contentWidth/2; i-- {
			if runes[i] == '/' {
				breakAt = i + 1
				break
			}
		}
		lines = append(lines, indent+string(runes[:breakAt]))
		runes = runes[breakAt:]
	}
	return lines
}

// Source implements navigator.Source for library browsing.
type Source struct {
	lib *Library
}

func NewSource(lib *Library) *Source {
	return &Source{lib: lib}
}

func (s *Source) Root() Node {
	return Node{
		level: LevelRoot,
		name:  "Library",
	}
}

func (s *Source) Children(parent Node) ([]Node, error) {
	switch parent.level {
	case LevelRoot:
		// Return list of artists
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

	case LevelArtist:
		// Return list of albums for this artist
		albums, err := s.lib.Albums(parent.artist)
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
				artist:    parent.artist,
				album:     album.Name,
				albumYear: album.Year,
				name:      name,
			}
		}
		return nodes, nil

	case LevelAlbum:
		// Return list of tracks for this album
		tracks, err := s.lib.Tracks(parent.artist, parent.album)
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
				artist: parent.artist,
				album:  parent.album,
				track:  track,
				name:   name,
			}
		}
		return nodes, nil

	case LevelTrack:
		// Tracks have no children
		return nil, nil
	}
	return nil, nil
}

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

func (s *Source) DisplayPath(node Node) string {
	root := icons.FormatDir("Library")
	switch node.level {
	case LevelRoot:
		return root
	case LevelArtist:
		return root + " > " + icons.FormatArtist(node.artist)
	case LevelAlbum:
		return root + " > " + icons.FormatArtist(node.artist) + " > " + icons.FormatAlbum(node.album)
	case LevelTrack:
		return root + " > " + icons.FormatArtist(node.artist) + " > " + icons.FormatAlbum(node.album)
	}
	return root
}

func (s *Source) NodeFromID(id string) (Node, bool) {
	parts := strings.SplitN(id, ":", 4)
	if len(parts) < 2 || parts[0] != "library" {
		return Node{}, false
	}

	switch parts[1] {
	case "root":
		return s.Root(), true
	case "artist":
		if len(parts) < 3 {
			return Node{}, false
		}
		return Node{
			level:  LevelArtist,
			artist: parts[2],
			name:   parts[2],
		}, true
	case "album":
		if len(parts) < 4 {
			return Node{}, false
		}
		return Node{
			level:  LevelAlbum,
			artist: parts[2],
			album:  parts[3],
			name:   parts[3],
		}, true
	case "track":
		if len(parts) < 3 {
			return Node{}, false
		}
		trackID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
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
