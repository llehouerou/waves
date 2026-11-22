package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// mkdirs makes each folder of paths, under dir.
func mkdirs(t *testing.T, dir string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Join(dir, p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
}

// lockFolder sets the mode of folder until the test ends, and skips the test
// for root, which reads every folder.
func lockFolder(t *testing.T, folder string, mode os.FileMode) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads every folder")
	}
	if err := os.Chmod(folder, mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o755) })
}

// newFilesModel is a model in the Files view, its navigator focused in folder
// on the entry selected.
func newFilesModel(t *testing.T, folder, selected string) Model {
	t.Helper()
	nav, err := openFileNav(folder)
	if err != nil {
		t.Fatalf("open file navigator: %v", err)
	}
	nav.FocusByName(selected)
	m := newIntegrationTestModel(t)
	m.Navigation.SetFileNav(nav)
	m.Navigation.SetViewMode(navctl.ViewFileBrowser)
	m.SetFocus(navctl.FocusNavigator)
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// assertStayedIn checks that the Files view is still in folder, on selected,
// listing entries, and that the error popup tells the listing of unreadable
// failed.
func assertStayedIn(t *testing.T, m Model, folder, selected, unreadable string, entries ...string) {
	t.Helper()
	nav := m.Navigation.FileNav()
	if got := nav.CurrentPath(); got != folder {
		t.Errorf("file navigator in %q, want %q", got, folder)
	}
	if got := nav.SelectedName(); got != selected {
		t.Errorf("file navigator on %q, want %q", got, selected)
	}
	view := testutil.StripANSI(nav.View())
	for _, entry := range entries {
		if !strings.Contains(view, entry) {
			t.Errorf("the Files view does not list %q:\n%s", entry, view)
		}
	}
	want := errmsg.Format(errmsg.OpNavigate, &os.PathError{Op: "open", Path: unreadable, Err: os.ErrPermission})
	if msg := m.Popups.ErrorMsg(); msg != want {
		t.Errorf("error popup = %q, want %q", msg, want)
	}
}

// Entering a folder that cannot be read leaves the Files view where it was
// and tells why.
func TestFiles_EnteringAFolderThatCannotBeReadStaysAndTellsWhy(t *testing.T) {
	for _, key := range []string{"l", keyEnter} {
		t.Run(key, func(t *testing.T) {
			dir := t.TempDir()
			mkdirs(t, dir, "music/album", "music/locked", "other")
			music := filepath.Join(dir, "music")
			locked := filepath.Join(music, "locked")
			lockFolder(t, locked, 0o000)
			m := newFilesModel(t, music, "locked")

			m = feed(t, m, keyMsg(key))

			assertStayedIn(t, m, music, "locked", locked, "album", "locked", "other")
		})
	}
}

// Going up to a folder that can be traversed but not read leaves the Files
// view where it was and tells why.
func TestFiles_GoingUpToAFolderThatCannotBeReadStaysAndTellsWhy(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "top/gate/inner/disc1", "top/gate/inner/disc2", "top/sibling")
	gate := filepath.Join(dir, "top", "gate")
	inner := filepath.Join(gate, "inner")
	lockFolder(t, gate, 0o311)
	m := newFilesModel(t, inner, "disc2")

	m = feed(t, m, keyMsg("h"))

	assertStayedIn(t, m, inner, "disc2", gate, "disc1", "disc2")
}
