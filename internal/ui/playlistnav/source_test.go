package playlistnav

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/llehouerou/waves/internal/core"
)

var errUnreadable = errors.New(`sql: Scan error on column index 3, name "created_at"`)

// fakePlaylists holds the folder 1, "Rock", and in it the playlist 1, "Mix",
// of two tracks. The reads of the folder 2 and of the playlist 2 fail with
// errUnreadable; any other folder or playlist is not there, and a playlist
// that is not there has no track.
type fakePlaylists struct{}

func (fakePlaylists) Folders(*int64) ([]core.PlaylistFolder, error) { return nil, nil }

func (fakePlaylists) List(*int64) ([]core.Playlist, error) { return nil, nil }

func (fakePlaylists) FolderByID(id int64) (*core.PlaylistFolder, error) {
	switch id {
	case 1:
		return &core.PlaylistFolder{ID: 1, Name: "Rock"}, nil
	case 2:
		return nil, errUnreadable
	}
	return nil, sql.ErrNoRows
}

func (fakePlaylists) Get(id int64) (*core.Playlist, error) {
	folder := int64(1)
	switch id {
	case 1:
		return &core.Playlist{ID: 1, FolderID: &folder, Name: "Mix"}, nil
	case 2:
		return nil, errUnreadable
	}
	return nil, sql.ErrNoRows
}

func (fakePlaylists) Tracks(id int64) ([]core.Track, error) {
	switch id {
	case 1:
		return []core.Track{
			{ID: 10, Artist: "Low", Title: "Candy Girl", TrackNumber: 1},
			{ID: 11, Artist: "Low", Title: "Lazy", TrackNumber: 3},
		}, nil
	case 2:
		return nil, errUnreadable
	}
	return nil, nil
}

func TestNodeFromID_GivesBackTheNodeItReads(t *testing.T) {
	src := NewSource(fakePlaylists{})

	if got, ok, err := src.NodeFromID("playlists:folder:1"); !ok || err != nil || got.Level() != LevelFolder || got.DisplayName() != "Rock" {
		t.Errorf("folder 1: level %v, name %q, ok %v, err %v; want the folder Rock", got.Level(), got.DisplayName(), ok, err)
	}
	if got, ok, err := src.NodeFromID("playlists:playlist:1"); !ok || err != nil || got.Level() != LevelPlaylist || got.DisplayName() != "Mix" {
		t.Errorf("playlist 1: level %v, name %q, ok %v, err %v; want the playlist Mix", got.Level(), got.DisplayName(), ok, err)
	}
	if got, ok, err := src.NodeFromID("playlists:track:1:1"); !ok || err != nil || got.Level() != LevelTrack || got.TrackID() != 11 || got.Position() != 1 {
		t.Errorf("track 1:1: level %v, track %d, position %d, ok %v, err %v; want Lazy at 1",
			got.Level(), got.TrackID(), got.Position(), ok, err)
	}
}

func TestNodeFromID_TellsANodeItCannotReadFromANodeThatIsNotThere(t *testing.T) {
	src := NewSource(fakePlaylists{})

	for _, id := range []string{"playlists:folder:2", "playlists:playlist:2", "playlists:track:2:0"} {
		if _, ok, err := src.NodeFromID(id); ok || !errors.Is(err, errUnreadable) {
			t.Errorf("%s: ok %v, err %v; want the failed read", id, ok, err)
		}
	}
	for _, id := range []string{
		"playlists:folder:3",   // gone
		"playlists:playlist:3", // gone
		"playlists:track:1:2",  // past the end
		"playlists:track:1:-1", // before the start
		"playlists:track:3:0",  // of a playlist gone
		"playlists:folder:x",   // cannot be parsed
		"playlists:track:1",    // cannot be parsed
		"library:artist:Low",   // of another source
	} {
		if _, ok, err := src.NodeFromID(id); ok || err != nil {
			t.Errorf("%s: ok %v, err %v; want no node and no error", id, ok, err)
		}
	}
}
