package librarynav

import "github.com/llehouerou/waves/internal/navigator/sourceutil"

// The IDs of the library's nodes: the only place that builds them and reads
// them back. An artist or an album name may hold any character.

const idPrefix = "library"

// The kinds of node an ID names, after its prefix.
const (
	kindRoot   = "root"
	kindArtist = "artist"
	kindAlbum  = "album"
	kindTrack  = "track"
)

// RootNodeID is the ID of the library's root.
func RootNodeID() string {
	return sourceutil.FormatID(idPrefix, kindRoot)
}

// ArtistNodeID is the ID of the node of an album artist.
func ArtistNodeID(artist string) string {
	return sourceutil.FormatID(idPrefix, kindArtist, artist)
}

// AlbumNodeID is the ID of the node of an album of an album artist.
func AlbumNodeID(artist, album string) string {
	return sourceutil.FormatID(idPrefix, kindAlbum, artist, album)
}

// TrackNodeID is the ID of the node of a library track.
func TrackNodeID(trackID int64) string {
	return sourceutil.FormatID(idPrefix, kindTrack, sourceutil.FormatInt64(trackID))
}

// parseID reads an ID built by one of the functions above. It gives the
// node of a root, an artist or an album; for a track, a node of level
// LevelTrack and nothing else, and the track's ID, for the source to read
// the track. Any other string, one with a part too many or too few
// included, is no ID.
func parseID(id string) (node Node, trackID int64, ok bool) {
	parts, ok := sourceutil.ParseID(id, idPrefix)
	if !ok {
		return Node{}, 0, false
	}
	switch {
	case parts[0] == kindRoot && len(parts) == 1:
		return Node{level: LevelRoot, name: libraryRootPath}, 0, true
	case parts[0] == kindArtist && len(parts) == 2:
		return Node{level: LevelArtist, artist: parts[1], name: parts[1]}, 0, true
	case parts[0] == kindAlbum && len(parts) == 3:
		return Node{level: LevelAlbum, artist: parts[1], album: parts[2], name: parts[2]}, 0, true
	case parts[0] == kindTrack && len(parts) == 2:
		trackID, ok := sourceutil.ParseInt64(parts[1])
		if !ok {
			return Node{}, 0, false
		}
		return Node{level: LevelTrack}, trackID, true
	}
	return Node{}, 0, false
}
