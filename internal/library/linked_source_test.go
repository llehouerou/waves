package library_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/library"
)

// link makes path a symbolic link to target.
func link(t *testing.T, target, path string) string {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	return path
}

// linkedSource returns a symbolic link, the source, to a directory holding
// a/one.mp3 and b/two.mp3, and that directory.
func linkedSource(t *testing.T) (source, target string) {
	t.Helper()
	root := t.TempDir()
	target = filepath.Join(root, "real")
	writeTrack(t, filepath.Join(target, "a", "one.mp3"), "One")
	writeTrack(t, filepath.Join(target, "b", "two.mp3"), "Two")
	return link(t, target, filepath.Join(root, "link")), target
}

func TestScan_WalksASourceThatIsALinkToADirectory(t *testing.T) {
	refreshSource := func(f scanFixture) refreshFunc {
		return func(sources []string, progress chan<- library.ScanProgress, stop func() bool) error {
			return f.lib.RefreshSource(sources[0], progress, stop)
		}
	}
	cases := []struct {
		name    string
		refresh func(f scanFixture) refreshFunc
	}{
		{name: "Refresh", refresh: func(f scanFixture) refreshFunc { return f.lib.Refresh }},
		{name: "FullRefresh", refresh: func(f scanFixture) refreshFunc { return f.lib.FullRefresh }},
		{name: "RefreshSource", refresh: refreshSource},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newScanFixture(t)
			source, _ := linkedSource(t)
			f.add(t, source)

			stats := f.scanBy(t, tc.refresh(f), nil, source)

			one, two := filepath.Join(source, "a", "one.mp3"), filepath.Join(source, "b", "two.mp3")
			if got := f.ids(t, one, two); slices.Contains(got, 0) {
				t.Errorf("track IDs of %s and %s = %v, want both tracks", one, two, got)
			}
			got := stats.BySource[source]
			if want := []string{"a/one.mp3", "b/two.mp3"}; !slices.Equal(sorted(got.Added), want) || len(got.Unread) != 0 {
				t.Errorf("stats = %+v, want %v added and nothing unread", got, want)
			}
		})
	}
}

func TestRefresh_KeepsTheTracksOfASourceReplacedByALinkToItsNewPlace(t *testing.T) {
	f := newScanFixture(t)
	root := t.TempDir()
	source := filepath.Join(root, "music")
	one := writeTrack(t, filepath.Join(source, "a", "one.mp3"), "One")
	two := writeTrack(t, filepath.Join(source, "b", "two.mp3"), "Two")
	f.scan(t, source)
	ids := f.ids(t, one, two)
	mix := f.playlistOf(t, one, two)

	moved := filepath.Join(root, "moved")
	if err := os.Rename(source, moved); err != nil {
		t.Fatalf("rename: %v", err)
	}
	link(t, moved, source)
	if err := os.Remove(filepath.Join(moved, "b", "two.mp3")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	three := filepath.Join(source, "a", "three.mp3")
	writeTrack(t, filepath.Join(moved, "a", "three.mp3"), "Three")
	stats := f.scan(t, source)

	if got := f.ids(t, one, two); !slices.Equal(got, []int64{ids[0], 0}) {
		t.Errorf("track IDs = %v, want %v: one unchanged, two removed", got, []int64{ids[0], 0})
	}
	if got := f.entries(t, mix); !slices.Equal(got, []string{one}) {
		t.Errorf("playlist = %v, want one left", got)
	}
	if f.trackID(t, three) == 0 {
		t.Errorf("no track for %s", three)
	}
	got := stats.BySource[source]
	if !slices.Equal(got.Added, []string{"a/three.mp3"}) || !slices.Equal(got.Removed, []string{"b/two.mp3"}) ||
		len(got.Unread) != 0 {
		t.Errorf("stats = %+v, want a/three.mp3 added, b/two.mp3 removed and nothing unread", got)
	}
}

func TestRefresh_KeepsTheTracksOfALinkSourceWhoseTargetIsGone(t *testing.T) {
	f := newScanFixture(t)
	source, target := linkedSource(t)
	f.scan(t, source)
	one, two := filepath.Join(source, "a", "one.mp3"), filepath.Join(source, "b", "two.mp3")
	ids := f.ids(t, one, two)
	mix := f.playlistOf(t, one, two)

	away := target + ".away"
	if err := os.Rename(target, away); err != nil {
		t.Fatalf("rename: %v", err)
	}
	stats := f.scan(t, source)

	if got := f.ids(t, one, two); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := f.entries(t, mix); !slices.Equal(got, []string{one, two}) {
		t.Errorf("playlist = %v, want both tracks kept", got)
	}
	if got := stats.BySource[source]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want the source unread and nothing removed", got)
	}

	if err := os.Rename(away, target); err != nil {
		t.Fatalf("rename back: %v", err)
	}
	stats = f.scan(t, source)

	if got := f.ids(t, one, two); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := stats.BySource[source]; len(got.Unread) != 0 || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want nothing unread or removed", got)
	}
}

func TestRefresh_KeepsTheTracksOfALinkSourceWithNoMusicFileLeft(t *testing.T) {
	f := newScanFixture(t)
	source, target := linkedSource(t)
	f.scan(t, source)
	one, two := filepath.Join(source, "a", "one.mp3"), filepath.Join(source, "b", "two.mp3")
	ids := f.ids(t, one, two)

	emptyOfMusic(t, target)
	stats := f.scan(t, source)

	if got := f.ids(t, one, two); !slices.Equal(got, ids) {
		t.Errorf("track IDs = %v, want %v kept", got, ids)
	}
	if got := stats.BySource[source]; !slices.Equal(got.Unread, []string{library.SourceRoot}) || len(got.Removed) != 0 {
		t.Errorf("stats = %+v, want the source unread and nothing removed", got)
	}
}

func TestRefresh_DoesNotFollowALinkToAFolderInsideASource(t *testing.T) {
	f := newScanFixture(t)
	root := t.TempDir()
	music := filepath.Join(root, "music")
	top := writeTrack(t, filepath.Join(music, "top.mp3"), "Top")
	outside := filepath.Join(root, "outside")
	writeTrack(t, filepath.Join(outside, "one.mp3"), "One")
	link(t, outside, filepath.Join(music, "linked"))

	stats := f.scan(t, music)

	if got := stats.BySource[music]; !slices.Equal(got.Added, []string{"top.mp3"}) {
		t.Errorf("added = %v, want top.mp3 alone", got.Added)
	}
	if f.trackID(t, top) == 0 {
		t.Errorf("no track for %s", top)
	}
	if id := f.trackID(t, filepath.Join(music, "linked", "one.mp3")); id != 0 {
		t.Errorf("track %d for the file behind the link, want none", id)
	}
}

func TestRefresh_WalksALinkSourceInsideAnotherSourceAsTheInnerSource(t *testing.T) {
	f := newScanFixture(t)
	root := t.TempDir()
	music := filepath.Join(root, "music")
	writeTrack(t, filepath.Join(music, "top.mp3"), "Top")
	elsewhere := filepath.Join(root, "elsewhere")
	writeTrack(t, filepath.Join(elsewhere, "a", "one.mp3"), "One")
	nas := link(t, elsewhere, filepath.Join(music, "nas"))

	stats := f.scan(t, music, nas)

	if got := stats.BySource[music].Added; !slices.Equal(got, []string{"top.mp3"}) {
		t.Errorf("added to music = %v, want top.mp3 alone", got)
	}
	if got := stats.BySource[nas].Added; !slices.Equal(got, []string{"a/one.mp3"}) {
		t.Errorf("added to nas = %v, want a/one.mp3", got)
	}
	if f.trackID(t, filepath.Join(nas, "a", "one.mp3")) == 0 {
		t.Errorf("no track for the file under nas")
	}
}
