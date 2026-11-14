package playlists

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// openFavorites opens a database holding a library track for each title, and
// favorites them in that order. It returns the playlists and the track IDs by
// title.
func openFavorites(t *testing.T, titles ...string) (*state.Manager, *Playlists, map[string]int64) {
	t.Helper()
	mgr, err := state.OpenAt(filepath.Join(t.TempDir(), "waves.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { mgr.Close() })
	lib := library.New(mgr.DB())
	p := New(mgr.DB(), lib)

	source := t.TempDir()
	for _, title := range titles {
		tagstest.WriteMP3(t, filepath.Join(source, title+".mp3"), tags.Tag{Title: title, Artist: "Artist", AlbumArtist: "Artist", Album: "Album"})
	}
	refresh(t, lib, source)

	ids := make(map[string]int64, len(titles))
	for _, title := range titles {
		track, err := lib.TrackByPath(filepath.Join(source, title+".mp3"))
		if err != nil {
			t.Fatalf("TrackByPath %s: %v", title, err)
		}
		ids[title] = track.ID
		if fav, err := p.ToggleFavorite(track.ID); err != nil || !fav {
			t.Fatalf("ToggleFavorite %s = %v, %v, want true", title, fav, err)
		}
	}
	return mgr, p, ids
}

func unfavorite(t *testing.T, p *Playlists, trackID int64) {
	t.Helper()
	fav, err := p.ToggleFavorite(trackID)
	if err != nil {
		t.Fatalf("ToggleFavorite: %v", err)
	}
	if fav {
		t.Fatalf("ToggleFavorite = true, want false")
	}
}

// After a track is un-favorited, removing a favorite by its index removes the
// track at that index.
func TestFavorites_RemoveAfterUnfavoriteRemovesTheTrackAtItsIndex(t *testing.T) {
	_, p, ids := openFavorites(t, "A", "B", "C", "D")

	unfavorite(t, p, ids["B"])

	if err := p.RemoveTrack(FavoritesPlaylistID, 2); err != nil {
		t.Fatalf("RemoveTrack: %v", err)
	}
	assertTitles(t, p, FavoritesPlaylistID, "A", "C")
}

// After a track is un-favorited, moving a favorite by its index moves the
// track at that index.
func TestFavorites_MoveAfterUnfavoriteMovesTheTrackAtItsIndex(t *testing.T) {
	_, p, ids := openFavorites(t, "A", "B", "C", "D")

	unfavorite(t, p, ids["B"])

	got, err := p.MoveIndices(FavoritesPlaylistID, []int{1}, 1)
	if err != nil {
		t.Fatalf("MoveIndices: %v", err)
	}
	if !slices.Equal(got, []int{2}) {
		t.Errorf("MoveIndices = %v, want [2]", got)
	}
	assertTitles(t, p, FavoritesPlaylistID, "A", "D", "C")
}

// Un-favoriting a track leaves the other favorites at positions 0..n-1, in the
// order they had, wherever the track was and however many times.
func TestFavorites_UnfavoriteLeavesNoGap(t *testing.T) {
	tests := []struct {
		name      string
		duplicate bool // B favorited a second time, at the end
		remove    string
		want      []string
	}{
		{name: "first", remove: "A", want: []string{"B", "C", "D"}},
		{name: "middle", remove: "B", want: []string{"A", "C", "D"}},
		{name: "last", remove: "D", want: []string{"A", "B", "C"}},
		{name: "twice", duplicate: true, remove: "B", want: []string{"A", "C", "D"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, p, ids := openFavorites(t, "A", "B", "C", "D")
			if tt.duplicate {
				if err := p.AddTracks(FavoritesPlaylistID, []int64{ids["B"]}); err != nil {
					t.Fatalf("AddTracks: %v", err)
				}
				assertTitles(t, p, FavoritesPlaylistID, "A", "B", "C", "D", "B")
			}

			unfavorite(t, p, ids[tt.remove])

			assertTitles(t, p, FavoritesPlaylistID, tt.want...)
			if got := positions(t, mgr, FavoritesPlaylistID); !slices.Equal(got, []int{0, 1, 2}) {
				t.Errorf("positions = %v, want [0 1 2]", got)
			}
		})
	}
}

// Un-favoriting the only favorite leaves Favorites empty.
func TestFavorites_UnfavoriteTheOnlyFavorite(t *testing.T) {
	mgr, p, ids := openFavorites(t, "A")

	unfavorite(t, p, ids["A"])

	assertTitles(t, p, FavoritesPlaylistID)
	if got := positions(t, mgr, FavoritesPlaylistID); len(got) != 0 {
		t.Errorf("positions = %v, want none", got)
	}
	if fav, err := p.IsFavorite(ids["A"]); err != nil || fav {
		t.Errorf("IsFavorite = %v, %v, want false", fav, err)
	}
}

// A track favorited again goes at the end of Favorites, right after the last one.
func TestFavorites_FavoriteAgainAppendsAtTheEnd(t *testing.T) {
	mgr, p, ids := openFavorites(t, "A", "B", "C", "D")
	unfavorite(t, p, ids["B"])

	if fav, err := p.ToggleFavorite(ids["B"]); err != nil || !fav {
		t.Fatalf("ToggleFavorite = %v, %v, want true", fav, err)
	}

	assertTitles(t, p, FavoritesPlaylistID, "A", "C", "D", "B")
	if got := positions(t, mgr, FavoritesPlaylistID); !slices.Equal(got, []int{0, 1, 2, 3}) {
		t.Errorf("positions = %v, want [0 1 2 3]", got)
	}
}
