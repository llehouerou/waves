package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/navigator"
	"github.com/llehouerou/waves/internal/state"
)

// savedEntry is the entry the saved navigation state selects in its folder.
const savedEntry = "b"

// newSavedFolderModel is a model that waits for its initialization over a
// database that an earlier run left in the Files view, in folder with
// savedEntry selected, and with defaultFolder as its default_folder.
func newSavedFolderModel(t *testing.T, folder, defaultFolder string) Model {
	t.Helper()
	database := filepath.Join(t.TempDir(), "waves.db")
	seedNavigation(t, database, state.NavigationState{
		CurrentPath:  folder,
		SelectedName: savedEntry,
		ViewMode:     string(navctl.ViewFileBrowser),
	})
	return newWaitingModel(t, database, defaultFolder)
}

// folderWithEntries is a folder holding the folders a and savedEntry: a
// navigator in it starts on a, unless it selects savedEntry.
func folderWithEntries(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"a", savedEntry} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	return dir
}

// unreadableFolder is a folder that exists and cannot be listed until the
// test ends.
func unreadableFolder(t *testing.T) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads every folder")
	}
	dir := filepath.Join(folderWithEntries(t), "locked")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	return dir
}

// initialized runs the model's initialization and checks that it succeeds.
func initialized(t *testing.T, m Model) InitResult {
	t.Helper()
	init, ok := m.startInitialization()().(InitResult)
	if !ok || init.Err != nil {
		t.Fatalf("initialization = %+v, want it to succeed", init)
	}
	return init
}

// assertFilesStartIn checks that the file navigator starts in folder on the
// entry selected, and the rest of the saved navigation state is restored.
func assertFilesStartIn(t *testing.T, init InitResult, folder, selected string) {
	t.Helper()
	fileNav, ok := init.FileNav.(navigator.Model[navigator.FileNode])
	if !ok {
		t.Fatalf("file navigator = %T, want navigator.Model[navigator.FileNode]", init.FileNav)
	}
	if got := fileNav.CurrentPath(); got != folder {
		t.Errorf("file navigator in %q, want %q", got, folder)
	}
	if got := fileNav.SelectedName(); got != selected {
		t.Errorf("file navigator on %q, want %q", got, selected)
	}
	if init.SavedView != navctl.ViewFileBrowser {
		t.Errorf("saved view = %q, want %q", init.SavedView, navctl.ViewFileBrowser)
	}
	if init.IsFirstLaunch {
		t.Error("IsFirstLaunch = true, want false: a navigation state was saved")
	}
}

// A saved folder that cannot be listed is passed over, as a removed one is:
// the Files view starts in default_folder, on its first entry.
func TestStartup_UnreadableSavedFolderStartsInDefaultFolder(t *testing.T) {
	defaultFolder := folderWithEntries(t)
	m := newSavedFolderModel(t, unreadableFolder(t), defaultFolder)

	init := initialized(t, m)

	assertFilesStartIn(t, init, defaultFolder, "a")
}

// With no default_folder, a saved folder that cannot be listed gives way to
// the current directory.
func TestStartup_UnreadableSavedFolderStartsInCurrentDirectory(t *testing.T) {
	cwd := folderWithEntries(t)
	t.Chdir(cwd)
	m := newSavedFolderModel(t, unreadableFolder(t), "")

	init := initialized(t, m)

	assertFilesStartIn(t, init, cwd, "a")
}

// A default_folder that does not exist still fails startup when it stands in
// for a saved folder that cannot be listed.
func TestStartup_UnreadableSavedFolderAndMissingDefaultFolderFail(t *testing.T) {
	m := newSavedFolderModel(t, unreadableFolder(t), "/no/such/folder")

	init, ok := m.startInitialization()().(InitResult)

	if !ok || !errors.Is(init.Err, fs.ErrNotExist) {
		t.Fatalf("initialization = %+v, want it to fail on the missing default_folder", init)
	}
}

// A saved folder that can be listed is where the Files view starts, on the
// saved entry.
func TestStartup_ReadableSavedFolderStartsOnTheSavedEntry(t *testing.T) {
	saved := folderWithEntries(t)
	m := newSavedFolderModel(t, saved, folderWithEntries(t))

	init := initialized(t, m)

	assertFilesStartIn(t, init, saved, savedEntry)
}

// A saved folder that was removed gives way to default_folder.
func TestStartup_RemovedSavedFolderStartsInDefaultFolder(t *testing.T) {
	defaultFolder := folderWithEntries(t)
	m := newSavedFolderModel(t, filepath.Join(t.TempDir(), "removed"), defaultFolder)

	init := initialized(t, m)

	assertFilesStartIn(t, init, defaultFolder, "a")
}
