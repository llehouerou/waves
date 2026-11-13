package playlists

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// A library track that leaves the library leaves every playlist with it, and
// the playlist keeps addressing its tracks by their index in Tracks.
func TestPlaylist_ALibraryTrackThatLeavesLeavesNoGap(t *testing.T) {
	removals := map[string]func(t *testing.T, lib *library.Library, kept, gone, goneFile string, goneID int64){
		"by id": func(t *testing.T, lib *library.Library, _, _, _ string, goneID int64) {
			if err := lib.DeleteTrack(goneID); err != nil {
				t.Fatalf("DeleteTrack: %v", err)
			}
		},
		"with its source": func(t *testing.T, lib *library.Library, _, gone, _ string, _ int64) {
			if err := lib.RemoveSource(gone); err != nil {
				t.Fatalf("RemoveSource: %v", err)
			}
		},
		"by a scan": func(t *testing.T, lib *library.Library, kept, gone, goneFile string, _ int64) {
			if err := os.Remove(goneFile); err != nil {
				t.Fatalf("remove file: %v", err)
			}
			refresh(t, lib, kept, gone)
		},
	}

	for name, remove := range removals {
		t.Run(name, func(t *testing.T) {
			mgr, err := state.OpenAt(filepath.Join(t.TempDir(), "waves.db"))
			if err != nil {
				t.Fatalf("OpenAt: %v", err)
			}
			defer mgr.Close()
			lib := library.New(mgr.DB())
			p := New(mgr.DB(), lib)

			// B alone in its own source, so that removing the source removes B only.
			kept, gone := t.TempDir(), t.TempDir()
			files := map[string]string{
				"A": filepath.Join(kept, "a.mp3"),
				"B": filepath.Join(gone, "b.mp3"),
				"C": filepath.Join(kept, "c.mp3"),
				"D": filepath.Join(kept, "d.mp3"),
			}
			for title, path := range files {
				tagstest.WriteMP3(t, path, tags.Tag{Title: title, Artist: "Artist", AlbumArtist: "Artist", Album: "Album"})
			}
			for _, src := range []string{kept, gone} {
				if err := lib.AddSource(src); err != nil {
					t.Fatalf("AddSource: %v", err)
				}
			}
			refresh(t, lib, kept, gone)

			ids := map[string]int64{}
			for title, path := range files {
				track, err := lib.TrackByPath(path)
				if err != nil {
					t.Fatalf("TrackByPath %s: %v", path, err)
				}
				ids[title] = track.ID
			}

			playlistID, err := p.Create(nil, "Mix")
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if err := p.AddTracks(playlistID, []int64{ids["A"], ids["B"], ids["C"], ids["D"]}); err != nil {
				t.Fatalf("AddTracks: %v", err)
			}

			remove(t, lib, kept, gone, files["B"], ids["B"])

			assertTitles(t, p, playlistID, "A", "C", "D")
			if n, err := p.TrackCount(playlistID); err != nil || n != 3 {
				t.Errorf("TrackCount = %d, %v, want 3", n, err)
			}
			if got := positions(t, mgr, playlistID); !slices.Equal(got, []int{0, 1, 2}) {
				t.Errorf("positions = %v, want [0 1 2]", got)
			}

			if err := p.RemoveTrack(playlistID, 1); err != nil {
				t.Fatalf("RemoveTrack: %v", err)
			}
			assertTitles(t, p, playlistID, "A", "D")

			if _, err := p.MoveIndices(playlistID, []int{1}, -1); err != nil {
				t.Fatalf("MoveIndices: %v", err)
			}
			assertTitles(t, p, playlistID, "D", "A")
		})
	}
}

// A scan told to stop before it found the files of the library knows too
// little to tell a track whose file is gone: it removes none.
func TestPlaylist_AScanToldToStopRemovesNoTrack(t *testing.T) {
	mgr, err := state.OpenAt(filepath.Join(t.TempDir(), "waves.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer mgr.Close()
	lib := library.New(mgr.DB())
	p := New(mgr.DB(), lib)

	source := t.TempDir()
	for _, title := range []string{"A", "B"} {
		tagstest.WriteMP3(t, filepath.Join(source, title+".mp3"), tags.Tag{Title: title, Artist: "Artist", AlbumArtist: "Artist", Album: "Album"})
	}
	refresh(t, lib, source)
	ids := make([]int64, 0, 2)
	for _, title := range []string{"A", "B"} {
		track, err := lib.TrackByPath(filepath.Join(source, title+".mp3"))
		if err != nil {
			t.Fatalf("TrackByPath %s: %v", title, err)
		}
		ids = append(ids, track.ID)
	}
	playlistID, err := p.Create(nil, "Mix")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := p.AddTracks(playlistID, ids); err != nil {
		t.Fatalf("AddTracks: %v", err)
	}

	scan(t, lib, func() bool { return true }, source)

	assertTitles(t, p, playlistID, "A", "B")
}

func refresh(t *testing.T, lib *library.Library, sources ...string) {
	t.Helper()
	scan(t, lib, nil, sources...)
}

// scan runs a scan of sources, which asks stop whether to end early.
func scan(t *testing.T, lib *library.Library, stop func() bool, sources ...string) {
	t.Helper()
	progress := make(chan library.ScanProgress)
	done := make(chan error, 1)
	go func() { done <- lib.Refresh(sources, progress, stop) }()
	for range progress {
	}
	if err := <-done; err != nil {
		t.Fatalf("Refresh: %v", err)
	}
}

func assertTitles(t *testing.T, p *Playlists, playlistID int64, want ...string) {
	t.Helper()
	tracks, err := p.Tracks(playlistID)
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}
	got := make([]string, 0, len(tracks))
	for _, track := range tracks {
		got = append(got, track.Title)
	}
	if !slices.Equal(got, want) {
		t.Errorf("tracks = %v, want %v", got, want)
	}
}

func positions(t *testing.T, mgr *state.Manager, playlistID int64) []int {
	t.Helper()
	rows, err := mgr.DB().Query(`SELECT position FROM playlist_tracks WHERE playlist_id = ? ORDER BY position`, playlistID)
	if err != nil {
		t.Fatalf("positions: %v", err)
	}
	defer rows.Close()
	var got []int
	for rows.Next() {
		var pos int
		if err := rows.Scan(&pos); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, pos)
	}
	return got
}
