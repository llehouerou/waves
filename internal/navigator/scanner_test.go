package navigator

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// #338: a root whose own name starts with a dot is searched; the hidden
// folders and files under it stay out.
func TestScanDir_SearchesARootWithAHiddenName(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".music")
	for _, dir := range []string{"a", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"a/one.mp3", "a/.two.mp3", ".hidden/three.mp3"} {
		if err := os.WriteFile(filepath.Join(root, file), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	rels, _ := scanned(t, root)

	if want := []string{"a/", "a/one.mp3"}; !slices.Equal(rels, want) {
		t.Errorf("scan = %v, want %v", rels, want)
	}
}
