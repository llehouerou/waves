package navigator

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/linkfs/linkfstest"
)

// listed returns the children of node as name/ for a folder and name for a
// file, in order, and their paths.
func listed(t *testing.T, src *FileSource, node FileNode) (names, paths []string) {
	t.Helper()
	children, err := src.Children(node)
	if err != nil {
		t.Fatalf("children of %s: %v", node.path, err)
	}
	for _, c := range children {
		name := c.name
		if c.isDir {
			name += "/"
		}
		names = append(names, name)
		paths = append(paths, c.path)
	}
	return names, paths
}

func TestFileSourceChildren_ListsALinkAsWhatItPointsTo(t *testing.T) {
	dir := linkfstest.Links(t)
	src, err := NewFileSource(dir)
	if err != nil {
		t.Fatalf("new file source: %v", err)
	}

	names, _ := listed(t, src, src.Root())

	if want := []string{"link/", "real/", "linked.mp3"}; !slices.Equal(names, want) {
		t.Errorf("children = %v, want %v", names, want)
	}
}

func TestFileSourceChildren_ListsTheContentOfALinkToAFolderUnderTheLink(t *testing.T) {
	dir := linkfstest.Links(t)
	src, err := NewFileSource(dir)
	if err != nil {
		t.Fatalf("new file source: %v", err)
	}

	names, paths := listed(t, src, FileNode{path: filepath.Join(dir, "link"), name: "link", isDir: true})

	want := []string{filepath.Join(dir, "link", "a")}
	if !slices.Equal(names, []string{"a/"}) || !slices.Equal(paths, want) {
		t.Errorf("children = %v at %v, want [a/] at %v", names, paths, want)
	}
}

func TestFileSourceChildren_ListsTheLinkAFileSourceStartedAt(t *testing.T) {
	dir := linkfstest.Links(t)
	src, err := NewFileSource(filepath.Join(dir, "link"))
	if err != nil {
		t.Fatalf("new file source: %v", err)
	}
	parent := src.Parent(src.Root())
	if parent == nil {
		t.Fatal("no parent of the root")
	}

	names, _ := listed(t, src, *parent)

	if !slices.Contains(names, "link/") {
		t.Errorf("children of the parent = %v, want link/ among them", names)
	}
}

// scanned runs ScanDir on root to its end and returns its items as RelPath/
// for a folder and RelPath for a file, sorted, and their paths in that order.
func scanned(t *testing.T, root string) (rels, paths []string) {
	t.Helper()
	var items []FileItem
	for result := range ScanDir(context.Background(), root) {
		if !result.Done {
			continue
		}
		for _, item := range result.Items {
			fileItem, ok := item.(FileItem)
			if !ok {
				t.Fatalf("item %T, want a FileItem", item)
			}
			items = append(items, fileItem)
		}
	}
	slices.SortFunc(items, func(a, b FileItem) int { return strings.Compare(a.RelPath, b.RelPath) })
	for _, item := range items {
		rel := item.RelPath
		if item.IsDir {
			rel += "/"
		}
		rels = append(rels, rel)
		paths = append(paths, item.Path)
	}
	return rels, paths
}

func TestScanDir_WalksALinkToAFolderAsThatFolder(t *testing.T) {
	dir := linkfstest.Links(t)

	for _, name := range []string{"real", "link"} {
		root := filepath.Join(dir, name)
		rels, paths := scanned(t, root)

		want := []string{filepath.Join(root, "a"), filepath.Join(root, "a", "one.mp3")}
		if !slices.Equal(rels, []string{"a/", "a/one.mp3"}) || !slices.Equal(paths, want) {
			t.Errorf("scan of %s = %v at %v, want [a/ a/one.mp3] at %v", name, rels, paths, want)
		}
	}
}

func TestScanDir_FindsALinkBelowTheRootAsWhatItPointsTo(t *testing.T) {
	dir := linkfstest.Links(t)

	rels, _ := scanned(t, dir)

	want := []string{"link/", "linked.mp3", "real/", "real/a/", "real/a/one.mp3"}
	if !slices.Equal(rels, want) {
		t.Errorf("scan = %v, want %v", rels, want)
	}
}

func TestScanDir_DoesNotFollowALinkToAFolderBelowTheRoot(t *testing.T) {
	top := linkfstest.Loops(t)

	rels, _ := scanned(t, top)

	if want := []string{"a/", "a/one.mp3", "inner/", "loop/"}; !slices.Equal(rels, want) {
		t.Errorf("scan = %v, want %v", rels, want)
	}
}
