package library_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
)

// TestRemoveSource_SelectsItsOwnTracksOnly removes a source next to another
// whose path would match it as a LIKE or a GLOB pattern, or with its case
// ignored: only the removed source's track is counted and removed.
func TestRemoveSource_SelectsItsOwnTracksOnly(t *testing.T) {
	cases := []struct{ removed, kept string }{
		{"Music", "music"},
		{"my_music", "my-music"},
		{"100%", "100%live"},
		{"a*", "ab"},
		{"[ab]", "a"},
		{`a\_b`, "a_b"},
	}
	for _, tc := range cases {
		t.Run(tc.removed+" next to "+tc.kept, func(t *testing.T) {
			f := newScanFixture(t)
			root := t.TempDir()
			removed, kept := filepath.Join(root, tc.removed), filepath.Join(root, tc.kept)
			gone := filepath.Join(removed, "gone.mp3")
			stay := filepath.Join(kept, "stay.mp3")
			tagstest.WriteMP3(t, gone, tags.Tag{Artist: "Gone", AlbumArtist: "Gone", Album: "Lost", Title: "Bye"})
			tagstest.WriteMP3(t, stay, tags.Tag{Artist: "Stay", AlbumArtist: "Stay", Album: "Found", Title: "Hello"})
			for _, src := range []string{removed, kept} {
				if _, err := f.lib.AddSource(src); err != nil {
					t.Fatalf("add source: %v", err)
				}
			}
			f.scan(t, removed, kept)
			if err := f.lib.RebuildFTSIndex(); err != nil {
				t.Fatalf("rebuild search index: %v", err)
			}
			stayID := f.trackID(t, stay)
			mix := f.playlistOf(t, gone, stay)

			for _, src := range []string{removed, removed + string(filepath.Separator)} {
				if count, err := f.lib.TrackCountBySource(src); err != nil || count != 1 {
					t.Errorf("TrackCountBySource(%q) = %d (err %v), want 1", src, count, err)
				}
			}

			if err := f.lib.RemoveSource(removed); err != nil {
				t.Fatalf("remove source: %v", err)
			}

			if f.trackID(t, gone) != 0 {
				t.Error("the track of the removed source is still in the library")
			}
			if got := f.trackID(t, stay); got != stayID {
				t.Errorf("track ID of the kept source = %d, want %d", got, stayID)
			}
			if got := f.entries(t, mix); !slices.Equal(got, []string{stay}) {
				t.Errorf("playlist = %v, want the kept track alone", got)
			}
			if err := f.playlists.RemoveTrack(mix, 0); err != nil {
				t.Fatalf("remove the track at position 0: %v", err)
			}
			if got := f.entries(t, mix); len(got) != 0 {
				t.Errorf("playlist after removing position 0 = %v, want the kept track to have been there", got)
			}
			if sources, err := f.lib.Sources(); err != nil || !slices.Equal(sources, []string{kept}) {
				t.Errorf("sources = %v (err %v), want %v", sources, err, []string{kept})
			}
			results, err := f.lib.SearchFTS("")
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			want := []string{"artist Stay", "album Stay/Found", "track Hello"}
			if got := describe(results); !slices.Equal(got, want) {
				t.Errorf("search index = %v, want %v", got, want)
			}
		})
	}
}

// describe names each search result by its type and what it shows, in a
// stable order: artists, albums, tracks.
func describe(results []library.SearchResult) []string {
	slices.SortStableFunc(results, func(a, b library.SearchResult) int { return int(a.Type) - int(b.Type) })
	out := make([]string, 0, len(results))
	for _, r := range results {
		switch r.Type {
		case library.ResultArtist:
			out = append(out, "artist "+r.Artist)
		case library.ResultAlbum:
			out = append(out, "album "+r.Artist+"/"+r.Album)
		case library.ResultTrack:
			out = append(out, "track "+r.TrackTitle)
		}
	}
	return out
}
