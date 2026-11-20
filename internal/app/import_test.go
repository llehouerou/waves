package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/tags/tagstest"
	dlview "github.com/llehouerou/waves/internal/ui/downloads"
	"github.com/llehouerou/waves/internal/ui/importpopup"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// startImportOfLowTrust starts, from the downloads view, the import of a
// download of one file as "Low" - "Trust", as the user does: the downloads
// view opens the popup, the core plans the import, two Enter start it. A
// release without an ID has no cover to fetch: the test stays offline.
func startImportOfLowTrust(t *testing.T, m Model) Model {
	t.Helper()
	d := bobsAlbum()
	d.Files = []core.DownloadFile{{Filename: `@@bob\Music\Album\01.mp3`, Size: 3}}
	d.MBReleaseDetails = &core.MBReleaseDetails{
		Release: core.MBRelease{Title: "Trust", Artist: "Low", ArtistID: "low", Status: "Official"},
		Tracks:  []core.MBTrack{{Position: 1, Title: "Canada"}},
	}
	d.MBReleaseGroup = &core.MBReleaseGroup{ID: "rg", PrimaryType: "Album"}
	id, err := m.Core.Downloads().Queue(d)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	d.ID = id
	tagstest.WriteMP3(t, filepath.Join(m.Slskd.CompletedPath, "Album", "01.mp3"), core.FileTags{})
	m.Navigation.SetViewMode(navctl.ViewDownloads)

	m, cmd := updateModel(t, m, dlview.ActionMsg(dlview.OpenImport{Download: &d}))
	if m.Popups.Import() == nil {
		t.Fatal("the import popup did not open")
	}
	msgs, _ := drain(t, cmd) // the core plans the import, off the UI goroutine
	if len(msgs) != 1 {
		t.Fatalf("opening the popup gave %#v, want the plan of the import", msgs)
	}
	m, _ = updateModel(t, m, msgs[0])
	m, _ = updateModel(t, m, keyMsg(keyEnter)) // path preview
	m, _ = updateModel(t, m, keyMsg(keyEnter)) // start the import
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Fatalf("starting the import: %s", msg)
	}
	return m
}

// The whole import, as the user goes through it: the downloads view opens the
// popup, the core plans the import, two Enter start it, and the core's events
// take the TUI to the imported album. Nothing here imports a file.
func TestImport_TheCoreRunsItAndTheTUIFollowsItToTheAlbum(t *testing.T) {
	m, _ := newDownloadsTestModel(t)
	if err := m.Core.Library().AddSource(t.TempDir()); err != nil {
		t.Fatalf("add source: %v", err)
	}
	m = startImportOfLowTrust(t, m)

	for m.Popups.Import() != nil {
		m = pumpUntil[ServiceImportMsg](t, m)
	}

	if m.Navigation.ViewMode() != navctl.ViewLibrary || m.Navigation.LibrarySubMode() != navctl.LibraryModeAlbum {
		t.Errorf("view = %v / %v, want the library's album view", m.Navigation.ViewMode(), m.Navigation.LibrarySubMode())
	}
	if album := m.Navigation.AlbumView().SelectedAlbum(); album == nil || album.AlbumArtist != "Low" || album.Album != "Trust" {
		t.Errorf("selected album = %+v, want Low - Trust", album)
	}
	if all, err := m.Core.Downloads().List(); err != nil || len(all) != 0 {
		t.Errorf("downloads = %+v (err %v), want the import finished", all, err)
	}
}

// The user closed the popup and stayed on the downloads view while the core
// imported: back on the album view, it lists the album imported.
func TestImport_EndedOffScreenTheAlbumViewListsTheAlbum(t *testing.T) {
	m, _ := newDownloadsTestModel(t)
	if err := m.Core.Library().AddSource(t.TempDir()); err != nil {
		t.Fatalf("add source: %v", err)
	}
	m = pumpUntil[LibraryScanCompleteMsg](t, m)
	m, _ = updateModel(t, m, keyMsg(keyEscape)) // the scan report
	m.Navigation.SetLibrarySubMode(navctl.LibraryModeAlbum)
	if err := m.Navigation.AlbumView().Refresh(); err != nil {
		t.Fatalf("album view: %v", err)
	}
	m = startImportOfLowTrust(t, m)
	m.Popups.Hide(popupctl.Import)

	m = pumpUntilMatch(t, m, func(msg tea.Msg) bool {
		imported, ok := msg.(ServiceImportMsg)
		_, finished := imported.Event.(core.ImportFinished)
		return ok && finished
	})
	if m.Navigation.ViewMode() != navctl.ViewDownloads {
		t.Fatalf("view = %v, want the downloads view kept", m.Navigation.ViewMode())
	}
	m.Navigation.SetViewMode(navctl.ViewLibrary)

	if !m.Navigation.AlbumView().SelectAlbum("Low", "Trust") {
		t.Error("the album view does not list Low - Trust")
	}
}

// done is the end of an import of download 7 whose one file ended in state.
func done(state core.FileState) core.ImportFinished {
	file := core.FileProgress{Path: "/done/Album/01.flac", State: state}
	if state == core.FileFailed {
		file.Err = errors.New("disk full")
	}
	return core.ImportFinished{DownloadID: 7, Artist: "Low", Album: "Trust", Files: []core.FileProgress{file}}
}

// The user closed the popup and went on: the end of the import must not take
// them away from what they are doing, but the views it changed are read again.
func TestImportFinished_WithThePopupClosedDoesNotNavigate(t *testing.T) {
	m, _ := newDownloadsTestModel(t)
	m.Navigation.SetViewMode(navctl.ViewDownloads)

	cmd := m.handleImportEvent(done(core.FileDone))

	if m.Navigation.ViewMode() != navctl.ViewDownloads {
		t.Errorf("view = %v, want the downloads view kept", m.Navigation.ViewMode())
	}
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want none", msg)
	}
	// Finish import dropped the download: the downloads view reads them again.
	msgs, _ := drain(t, cmd)
	if len(msgs) != 1 {
		t.Fatalf("got %#v, want the downloads read again", msgs)
	}
	if _, ok := msgs[0].(DownloadsChangedMsg); !ok {
		t.Errorf("got %#v, want the downloads read again", msgs[0])
	}
}

// With no popup left to show it, a failure is an error popup.
func TestImportFinished_WithThePopupClosedShowsAFailure(t *testing.T) {
	tests := []struct {
		name     string
		finished core.ImportFinished
		want     string
	}{
		{"a file failed", done(core.FileFailed), "Import of Low - Trust: 1 of 1 files failed"},
		{"the library was not updated", func() core.ImportFinished {
			e := done(core.FileDone)
			e.Err = errors.New("database is locked")
			return e
		}(), "Import of Low - Trust: library not updated: database is locked"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := newDownloadsTestModel(t)

			m.handleImportEvent(tt.finished)

			if got := m.Popups.ErrorMsg(); got != tt.want {
				t.Errorf("error popup = %q, want %q", got, tt.want)
			}
		})
	}
}

// The popup shows its own failures: no error popup on top of it.
func TestImportFinished_WithThePopupOpenLeavesAFailureToIt(t *testing.T) {
	m, _ := newDownloadsTestModel(t)
	m.Popups.ShowImport(&core.Download{ID: 7}, []string{"/music"}, m.Core.Downloads())

	m.handleImportEvent(done(core.FileFailed))

	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want the failure left to the import popup", msg)
	}
	if m.Popups.Import() == nil {
		t.Error("the import popup was closed on a failure")
	}
}

// What the popup asks the core when it opens reaches it through the model.
func TestImportPopup_GetsThePlanItAskedFor(t *testing.T) {
	m, _ := newDownloadsTestModel(t)
	cmd := m.Popups.ShowImport(&core.Download{ID: 7, MBArtistName: "Low", MBAlbumTitle: "Trust"}, []string{"/music"}, m.Core.Downloads())
	prepared, ok := cmd().(importpopup.PreparedMsg)
	if !ok || prepared.Err == nil {
		t.Fatalf("the core answered %#v, want no plan: there is no download 7", prepared)
	}

	m, _ = updateModel(t, m, prepared)

	if view := testutil.StripANSI(m.Popups.Import().View()); !strings.Contains(view, prepared.Err.Error()) {
		t.Errorf("the popup did not get the core's answer:\n%s", view)
	}
}
