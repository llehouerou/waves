package library

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/navigator/sourceutil"
)

// Level represents the hierarchy level of a library node.
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

// ID returns a unique identifier for this node.
func (n Node) ID() string {
	switch n.level {
	case LevelRoot:
		return sourceutil.FormatID("library", "root")
	case LevelArtist:
		return sourceutil.FormatID("library", "artist", n.artist)
	case LevelAlbum:
		return sourceutil.FormatID("library", "album", n.artist, n.album)
	case LevelTrack:
		if n.track != nil {
			return sourceutil.FormatID("library", "track", sourceutil.FormatInt64(n.track.ID))
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

// TrackID returns the library track ID for track nodes, 0 otherwise.
// Implements navigator.TrackIDProvider.
func (n Node) TrackID() int64 {
	if n.track != nil {
		return n.track.ID
	}
	return 0
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

// SearchItem represents a library item for search results (deep search).
type SearchItem struct {
	Result SearchResult
}

// FilterValue returns the searchable text for filtering.
func (s SearchItem) FilterValue() string {
	switch s.Result.Type {
	case ResultArtist:
		return s.Result.Artist
	case ResultAlbum:
		// Include artist for "low california" to match "Low > California"
		return s.Result.Artist + " " + s.Result.Album
	case ResultTrack:
		// Include artist and album for full path matching
		parts := []string{s.Result.Artist, s.Result.Album, s.Result.TrackTitle}
		if s.Result.TrackArtist != "" && s.Result.TrackArtist != s.Result.Artist {
			parts = append(parts, s.Result.TrackArtist)
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// DisplayText returns the display text for search results.
func (s SearchItem) DisplayText() string {
	switch s.Result.Type {
	case ResultArtist:
		return icons.FormatArtist(s.Result.Artist)
	case ResultAlbum:
		album := s.Result.Album
		if s.Result.AlbumYear > 0 {
			album = fmt.Sprintf("[%d] %s", s.Result.AlbumYear, album)
		}
		return icons.FormatArtist(s.Result.Artist) + " > " + icons.FormatAlbum(album)
	case ResultTrack:
		album := s.Result.Album
		if s.Result.AlbumYear > 0 {
			album = fmt.Sprintf("[%d] %s", s.Result.AlbumYear, album)
		}
		track := s.Result.TrackTitle
		// Add artist prefix if different from album artist
		if s.Result.TrackArtist != "" && s.Result.TrackArtist != s.Result.Artist {
			track = s.Result.TrackArtist + " - " + track
		}
		// Add track number like navigator
		if s.Result.TrackNumber > 0 {
			if s.Result.DiscNumber > 1 {
				track = fmt.Sprintf("%d.%02d. %s", s.Result.DiscNumber, s.Result.TrackNumber, track)
			} else {
				track = fmt.Sprintf("%02d. %s", s.Result.TrackNumber, track)
			}
		}
		return icons.FormatArtist(s.Result.Artist) + " > " + icons.FormatAlbum(album) + " > " + icons.FormatAudio(track)
	}
	return ""
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
	case LevelArtist:
		return icons.FormatArtist(n.Node.DisplayName())
	case LevelAlbum:
		return icons.FormatAlbum(n.Node.DisplayName())
	case LevelTrack:
		return icons.FormatAudio(n.Node.DisplayName())
	}
	return n.Node.DisplayName()
}
