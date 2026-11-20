package playlist

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/linkfs/linkfstest"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/tags/tagstest"
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

// writeFiles writes under dir a tagged MP3 at each of tracks, a file that is not
// audio at each of others, and returns dir.
func writeFiles(t *testing.T, dir string, tracks, others []string) string {
	t.Helper()
	for _, rel := range tracks {
		tagstest.WriteMP3(t, filepath.Join(dir, rel), tags.Tag{Artist: "Low", Album: "Trust", Title: rel})
	}
	for _, rel := range others {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("not audio"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// under joins each of rels to dir.
func under(dir string, rels ...string) []string {
	joined := make([]string, 0, len(rels))
	for _, rel := range rels {
		joined = append(joined, filepath.Join(dir, rel))
	}
	return joined
}

// #346: queueing a folder walks it as the Files view lists it.
func TestCollectFromPath_LeavesOutHiddenEntries(t *testing.T) {
	music := writeFiles(t, filepath.Join(t.TempDir(), "music"),
		[]string{"one.mp3", ".two.mp3", ".trash/a/old.mp3", "sub/.stversions/deep.mp3", "sub/three.mp3"},
		[]string{"._one.mp3"})

	tracks, err := CollectFromPath(music)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if got, want := paths(tracks), under(music, "one.mp3", "sub/three.mp3"); !slices.Equal(got, want) {
		t.Errorf("collect = %v, want %v", got, want)
	}
}

func TestCollectFromPath_WalksAFolderWhateverItsName(t *testing.T) {
	hidden := writeFiles(t, filepath.Join(t.TempDir(), ".hidden"), []string{"a/two.mp3", ".b/three.mp3"}, nil)
	nested := writeFiles(t, filepath.Join(t.TempDir(), ".config", "music"), []string{"one.mp3"}, nil)

	for dir, want := range map[string][]string{
		hidden: under(hidden, "a/two.mp3"),
		nested: under(nested, "one.mp3"),
	} {
		tracks, err := CollectFromPath(dir)
		if err != nil {
			t.Fatalf("collect %s: %v", dir, err)
		}
		if got := paths(tracks); !slices.Equal(got, want) {
			t.Errorf("collect %s = %v, want %v", dir, got, want)
		}
	}
}

func TestCollectFromPath_TakesASingleHiddenFile(t *testing.T) {
	dir := writeFiles(t, t.TempDir(), []string{".one.mp3"}, nil)
	hidden := filepath.Join(dir, ".one.mp3")

	tracks, err := CollectFromPath(hidden)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if got := paths(tracks); !slices.Equal(got, []string{hidden}) {
		t.Errorf("collect = %v, want [%s]", got, hidden)
	}
}

func TestCollectFolderFromPath_LeavesOutHiddenFilesNextToTheFile(t *testing.T) {
	music := writeFiles(t, filepath.Join(t.TempDir(), "music"), []string{"one.mp3", ".two.mp3"}, []string{"._one.mp3"})

	tracks, index, err := CollectFolderFromPath(filepath.Join(music, "one.mp3"))
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if got, want := paths(tracks), under(music, "one.mp3"); !slices.Equal(got, want) || index != 0 {
		t.Errorf("collect = %v at %d, want %v at 0", got, index, want)
	}
}

func TestCollectFolderFromPath_TakesAFolderWhateverItsName(t *testing.T) {
	hidden := writeFiles(t, filepath.Join(t.TempDir(), ".hidden"), []string{"one.mp3", "two.mp3"}, nil)

	tracks, index, err := CollectFolderFromPath(filepath.Join(hidden, "two.mp3"))
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if got, want := paths(tracks), under(hidden, "one.mp3", "two.mp3"); !slices.Equal(got, want) || index != 1 {
		t.Errorf("collect = %v at %d, want %v at 1", got, index, want)
	}
}

func TestCollectFolderFromPath_KeepsTheSelectedHiddenFile(t *testing.T) {
	music := writeFiles(t, filepath.Join(t.TempDir(), "music"), []string{"a.mp3", ".b.mp3", ".c.mp3", "d.mp3"}, nil)

	tracks, index, err := CollectFolderFromPath(filepath.Join(music, ".c.mp3"))
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if got, want := paths(tracks), under(music, ".c.mp3", "a.mp3", "d.mp3"); !slices.Equal(got, want) || index != 0 {
		t.Errorf("collect = %v at %d, want %v at 0", got, index, want)
	}
}

func TestCollectFolderFromPath_LeavesOutHiddenEntriesOfADirectory(t *testing.T) {
	music := writeFiles(t, filepath.Join(t.TempDir(), "music"),
		[]string{"one.mp3", ".two.mp3", ".trash/old.mp3", "sub/three.mp3"}, nil)

	tracks, index, err := CollectFolderFromPath(music)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if got, want := paths(tracks), under(music, "one.mp3", "sub/three.mp3"); !slices.Equal(got, want) || index != 0 {
		t.Errorf("collect = %v at %d, want %v at 0", got, index, want)
	}
}

func TestCollect_LeavesOutAHiddenLinkToAMusicFile(t *testing.T) {
	dir := linkfstest.Links(t)
	linkfstest.Link(t, filepath.Join(dir, "real", "a", "one.mp3"), filepath.Join(dir, "real", "a", ".hidden.mp3"))

	tracks, err := CollectFromPath(filepath.Join(dir, "real"))
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	want := under(dir, "real/a/one.mp3")
	if got := paths(tracks); !slices.Equal(got, want) {
		t.Errorf("collect = %v, want %v", got, want)
	}

	tracks, index, err := CollectFolderFromPath(filepath.Join(dir, "real", "a", "one.mp3"))
	if err != nil {
		t.Fatalf("collect folder: %v", err)
	}
	if got := paths(tracks); !slices.Equal(got, want) || index != 0 {
		t.Errorf("collect folder = %v at %d, want %v at 0", got, index, want)
	}
}

func TestCollectFolderFromPath_KeepsTheSelectedHiddenFileGivenByAnUncleanPath(t *testing.T) {
	music := writeFiles(t, filepath.Join(t.TempDir(), "music"), []string{"a.mp3", ".c.mp3"}, nil)

	tracks, index, err := CollectFolderFromPath(music + "/./.c.mp3")
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if got, want := paths(tracks), under(music, ".c.mp3", "a.mp3"); !slices.Equal(got, want) || index != 0 {
		t.Errorf("collect = %v at %d, want %v at 0", got, index, want)
	}
}
