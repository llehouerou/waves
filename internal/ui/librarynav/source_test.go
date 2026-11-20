package librarynav

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/llehouerou/waves/internal/core"
)

// fakeLibrary is a library of artists and the names of their albums, with no
// track.
type fakeLibrary map[string][]string

func (f fakeLibrary) Artists() ([]string, error) {
	artists := make([]string, 0, len(f))
	for artist := range f {
		artists = append(artists, artist)
	}
	return artists, nil
}

func (f fakeLibrary) Albums(albumArtist string) ([]core.Album, error) {
	albums := make([]core.Album, 0, len(f[albumArtist]))
	for _, name := range f[albumArtist] {
		albums = append(albums, core.Album{Name: name})
	}
	return albums, nil
}

func (f fakeLibrary) Tracks(string, string) ([]core.LibraryTrack, error) { return nil, nil }

func (f fakeLibrary) TrackByID(int64) (*core.LibraryTrack, error) {
	return nil, errors.New("no track")
}

func (f fakeLibrary) AlbumHasMultipleDiscs(string, string) (bool, error) { return false, nil }

// awkwardNames are artist names an ID must give back as they are.
var awkwardNames = []string{"AC:DC", "Artist: Subtitle", "100%3A Pure %", ":::", "Plain"}

func TestNodeFromID_GivesBackTheArtistAndTheAlbumOfTheNode(t *testing.T) {
	lib := fakeLibrary{}
	for _, artist := range awkwardNames {
		lib[artist] = []string{"Back in Black", "Live: 1975"}
	}
	src := NewSource(lib)
	artists, err := src.Children(src.Root())
	if err != nil {
		t.Fatalf("artists: %v", err)
	}
	if len(artists) != len(awkwardNames) {
		t.Fatalf("%d artists, want %d", len(artists), len(awkwardNames))
	}

	for _, artist := range artists {
		got, ok, err := src.NodeFromID(artist.ID())
		if !ok || err != nil || got.Level() != LevelArtist || got.Artist() != artist.Artist() {
			t.Errorf("artist %q: NodeFromID(%q) = level %v, artist %q, ok %v",
				artist.Artist(), artist.ID(), got.Level(), got.Artist(), ok)
		}
		albums, err := src.Children(artist)
		if err != nil {
			t.Fatalf("albums of %q: %v", artist.Artist(), err)
		}
		for _, album := range albums {
			got, ok, err := src.NodeFromID(album.ID())
			if !ok || err != nil || got.Level() != LevelAlbum || got.Artist() != artist.Artist() || got.Album() != album.Album() {
				t.Errorf("album %q of %q: NodeFromID(%q) = level %v, artist %q, album %q, ok %v",
					album.Album(), artist.Artist(), album.ID(), got.Level(), got.Artist(), got.Album(), ok)
			}
		}
	}
}

func TestNodeIDs_AreTheIDsOfTheNodesOfTheSource(t *testing.T) {
	lib := fakeLibrary{}
	for _, artist := range awkwardNames {
		lib[artist] = []string{"Live: 1975"}
	}
	src := NewSource(lib)
	artists, err := src.Children(src.Root())
	if err != nil {
		t.Fatalf("artists: %v", err)
	}
	for _, artist := range artists {
		if got := ArtistNodeID(artist.Artist()); got != artist.ID() {
			t.Errorf("ArtistNodeID(%q) = %q, want the ID of the artist, %q", artist.Artist(), got, artist.ID())
		}
		albums, err := src.Children(artist)
		if err != nil {
			t.Fatalf("albums of %q: %v", artist.Artist(), err)
		}
		if got := AlbumNodeID(artist.Artist(), albums[0].Album()); got != albums[0].ID() {
			t.Errorf("AlbumNodeID(%q, %q) = %q, want the ID of the album, %q",
				artist.Artist(), albums[0].Album(), got, albums[0].ID())
		}
	}
}

func TestNodeIDs_TwoAlbumsWithTheSameJoinedNamesHaveTwoIDs(t *testing.T) {
	src := NewSource(fakeLibrary{"AC": {"DC:Back"}, "AC:DC": {"Back"}})
	artists, err := src.Children(src.Root())
	if err != nil {
		t.Fatalf("artists: %v", err)
	}
	ids := map[string]string{}
	for _, artist := range artists {
		albums, err := src.Children(artist)
		if err != nil {
			t.Fatalf("albums of %q: %v", artist.Artist(), err)
		}
		id := albums[0].ID()
		if other, ok := ids[id]; ok {
			t.Errorf("the albums of %q and of %q have the same ID %q", other, artist.Artist(), id)
		}
		ids[id] = artist.Artist()
	}
}

func TestNodeFromID_ReadsNoIDWithAColonLeftInAName(t *testing.T) {
	src := NewSource(fakeLibrary{"AC:DC": {"Back in Black"}})
	for _, id := range []string{"library:artist:AC:DC", "library:album:AC:DC:Back in Black"} {
		if got, ok, err := src.NodeFromID(id); ok || err != nil {
			t.Errorf("NodeFromID(%q) = level %v, artist %q, album %q, err %v, want no node",
				id, got.Level(), got.Artist(), got.Album(), err)
		}
	}
}

// trackLibrary holds the track ID 1, by Low on "Trust", and the track ID 4,
// on "Broken", whose discs cannot be read; the read of the track ID 2 fails
// with errUnreadable, and any other track is not there.
type trackLibrary struct{ fakeLibrary }

var errUnreadable = errors.New(`sql: Scan error on column index 9, name "year"`)

func (trackLibrary) TrackByID(id int64) (*core.LibraryTrack, error) {
	switch id {
	case 1:
		return &core.LibraryTrack{ID: 1, AlbumArtist: "Low", Artist: "Low", Album: "Trust", Title: "Candy Girl", TrackNumber: 1}, nil
	case 2:
		return nil, errUnreadable
	case 4:
		return &core.LibraryTrack{ID: 4, AlbumArtist: "Low", Artist: "Low", Album: "Broken", Title: "Lazy", TrackNumber: 1}, nil
	}
	return nil, sql.ErrNoRows
}

func (trackLibrary) AlbumHasMultipleDiscs(_, album string) (bool, error) {
	if album == "Broken" {
		return false, errUnreadable
	}
	return false, nil
}

func TestNodeFromID_TellsATrackItCannotReadFromATrackThatIsNotThere(t *testing.T) {
	src := NewSource(trackLibrary{})

	if got, ok, err := src.NodeFromID(TrackNodeID(1)); !ok || err != nil || got.Level() != LevelTrack || got.Album() != "Trust" {
		t.Errorf("track 1: level %v, album %q, ok %v, err %v; want the track node of Trust", got.Level(), got.Album(), ok, err)
	}
	if _, ok, err := src.NodeFromID(TrackNodeID(2)); ok || !errors.Is(err, errUnreadable) {
		t.Errorf("track 2: ok %v, err %v; want the failed read", ok, err)
	}
	if _, ok, err := src.NodeFromID(TrackNodeID(3)); ok || err != nil {
		t.Errorf("track 3: ok %v, err %v; want no node and no error", ok, err)
	}
	if _, ok, err := src.NodeFromID(TrackNodeID(4)); ok || !errors.Is(err, errUnreadable) {
		t.Errorf("track 4: ok %v, err %v; want the failed read of its discs", ok, err)
	}
}
