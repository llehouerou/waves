package playlist

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/linkfs/linkfstest"
)

// paths returns the paths of tracks, in order.
func paths(tracks []Track) []string {
	got := make([]string, 0, len(tracks))
	for i := range tracks {
		got = append(got, tracks[i].Path)
	}
	return got
}

func TestCollectFromPath_WalksALinkToAFolderAsThatFolder(t *testing.T) {
	dir := linkfstest.Links(t)

	for _, name := range []string{"real", "link"} {
		tracks, err := CollectFromPath(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("collect %s: %v", name, err)
		}
		want := []string{filepath.Join(dir, name, "a", "one.mp3")}
		if got := paths(tracks); !slices.Equal(got, want) {
			t.Errorf("collect %s = %v, want %v", name, got, want)
		}
	}
}

func TestCollectFolderFromPath_WalksALinkToAFolderAsThatFolder(t *testing.T) {
	dir := linkfstest.Links(t)

	tracks, index, err := CollectFolderFromPath(filepath.Join(dir, "link"))
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	want := []string{filepath.Join(dir, "link", "a", "one.mp3")}
	if got := paths(tracks); !slices.Equal(got, want) || index != 0 {
		t.Errorf("collect = %v at %d, want %v at 0", got, index, want)
	}
}

func TestCollectFromPath_TakesALinkBelowTheRootForWhatItPointsTo(t *testing.T) {
	dir := linkfstest.Links(t)

	tracks, err := CollectFromPath(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	want := []string{filepath.Join(dir, "linked.mp3"), filepath.Join(dir, "real", "a", "one.mp3")}
	if got := paths(tracks); !slices.Equal(got, want) {
		t.Errorf("collect = %v, want %v", got, want)
	}
}

func TestCollectFolderFromPath_TakesTheLinksNextToTheFileForWhatTheyPointTo(t *testing.T) {
	dir := linkfstest.Links(t)
	linked := filepath.Join(dir, "linked.mp3")

	tracks, index, err := CollectFolderFromPath(linked)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if got := paths(tracks); !slices.Equal(got, []string{linked}) || index != 0 {
		t.Errorf("collect = %v at %d, want [%s] at 0", got, index, linked)
	}
}

func TestCollectFolderFromPath_KeepsTheSelectedFileThatCannotBeRead(t *testing.T) {
	dir := linkfstest.Links(t)
	dangling := filepath.Join(dir, "dangling.mp3")

	tracks, index, err := CollectFolderFromPath(dangling)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	want := []string{dangling, filepath.Join(dir, "linked.mp3")}
	if got := paths(tracks); !slices.Equal(got, want) || index != 0 {
		t.Errorf("collect = %v at %d, want %v at 0", got, index, want)
	}
}

func TestCollectFromPath_DoesNotFollowALinkToAFolderBelowTheRoot(t *testing.T) {
	top := linkfstest.Loops(t)

	tracks, err := CollectFromPath(top)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	want := []string{filepath.Join(top, "a", "one.mp3")}
	if got := paths(tracks); !slices.Equal(got, want) {
		t.Errorf("collect = %v, want %v", got, want)
	}
}
