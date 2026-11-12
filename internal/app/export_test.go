package app

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// newExportTestModel builds a model whose library holds Low's Trust, three
// files that exist, and which has one export target saved: the folder it
// returns, or a device that is not plugged in when onDevice is set.
func newExportTestModel(t *testing.T, onDevice bool) (m Model, dest string) {
	t.Helper()
	src, dest := t.TempDir(), t.TempDir()
	m = newLibraryTestModel(t, func(db *sql.DB) {
		for i := 1; i <= 3; i++ {
			path := filepath.Join(src, strconv.Itoa(i)+".mp3")
			if err := os.WriteFile(path, []byte("track "+strconv.Itoa(i)), 0o600); err != nil {
				t.Fatalf("write track: %v", err)
			}
			_, err := db.Exec(`
				INSERT INTO library_tracks
					(path, mtime, artist, album_artist, album, title, disc_number, track_number, year, added_at, updated_at)
				VALUES (?, 1000, 'Low', 'Low', 'Trust', ?, 1, ?, 2002, 1000, 1000)
			`, path, "Track "+strconv.Itoa(i), i)
			if err != nil {
				t.Fatalf("insert track: %v", err)
			}
		}
		uuid, subfolder := "", dest
		if onDevice {
			uuid, subfolder = "no-such-device", "/Music"
		}
		_, err := db.Exec(`
			INSERT INTO export_targets (name, device_uuid, device_label, subfolder, folder_structure)
			VALUES ('Backup', ?, '', ?, 'flat')
		`, uuid, subfolder)
		if err != nil {
			t.Fatalf("insert target: %v", err)
		}
	})
	return m, dest
}

// pumpUntilExport feeds the core's events to the model until it has handled
// one of type E about an export.
func pumpUntilExport[E core.Event](t *testing.T, m Model) Model {
	t.Helper()
	return pumpUntilMatch(t, m, func(msg tea.Msg) bool {
		e, ok := msg.(ServiceExportMsg)
		if !ok {
			return false
		}
		_, ok = e.Event.(E)
		return ok
	})
}

// The whole export, as the user goes through it: e on the artist selected in
// the library, Enter on the target, then the core runs it and the TUI follows.
func TestExport_TheCoreRunsItAndTheTUIFollowsIt(t *testing.T) {
	m, dest := newExportTestModel(t, false)

	m = press(t, m, "e")
	pop := m.Popups.Get(popupctl.Export)
	if pop == nil {
		t.Fatal("the export popup is not open")
	}
	if view := testutil.StripANSI(pop.View()); !strings.Contains(view, "Backup") || !strings.Contains(view, "3 tracks") {
		t.Fatalf("the popup does not show the target and the three tracks:\n%s", view)
	}

	m = press(t, m, keyEnter)
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Fatalf("starting the export: %s", msg)
	}
	if m.Popups.IsVisible(popupctl.Export) {
		t.Error("the export popup is still open once the export is started")
	}

	m = pumpUntilExport[core.ExportProgressed](t, m)
	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "Low - Trust → Backup") {
		t.Errorf("the job bar does not show the export:\n%s", view)
	}

	m = pumpUntilExport[core.ExportFinished](t, m)
	if n := m.ActiveJobCount(); n != 0 {
		t.Errorf("%d jobs in the job bar after the export", n)
	}
	if len(m.Notifications) != 1 || m.Notifications[0].Message != "Low - Trust → Backup (3 tracks)" {
		t.Errorf("notifications = %+v, want the export told once", m.Notifications)
	}
	for i := 1; i <= 3; i++ {
		name := "0" + strconv.Itoa(i) + " - Track " + strconv.Itoa(i) + ".mp3"
		if got, err := os.ReadFile(filepath.Join(dest, "Low - Trust", name)); err != nil || string(got) != "track "+strconv.Itoa(i) {
			t.Errorf("%s = %q (err %v), want the track", name, got, err)
		}
	}
}

func TestExport_AFailedExportIsToldInAnErrorPopup(t *testing.T) {
	m, _ := newExportTestModel(t, false)
	// The files are gone since the library last saw them.
	tracks, err := m.Core.Library().Tracks("Low", "Trust")
	if err != nil {
		t.Fatalf("tracks: %v", err)
	}
	for i := range tracks {
		if err := os.Remove(tracks[i].Path); err != nil {
			t.Fatal(err)
		}
	}
	m = press(t, m, "e")
	m = press(t, m, keyEnter)

	m = pumpUntilExport[core.ExportFinished](t, m)

	if msg := m.Popups.ErrorMsg(); !strings.Contains(msg, "Export failed") {
		t.Errorf("error popup = %q, want the failed export told", msg)
	}
	if n := m.ActiveJobCount(); n != 0 {
		t.Errorf("%d jobs in the job bar after the export", n)
	}
	if len(m.Notifications) != 0 {
		t.Errorf("notifications = %+v, want none for an export that failed", m.Notifications)
	}
}

// The core refuses the target: the popup stays, for the user to pick another
// or plug the device in.
func TestExport_ATargetWhoseDeviceIsNotConnectedIsRefused(t *testing.T) {
	m, _ := newExportTestModel(t, true)
	m = press(t, m, "e")

	m = press(t, m, keyEnter)

	if msg := m.Popups.ErrorMsg(); msg != "Device for Backup is not connected" {
		t.Errorf("error popup = %q, want the device said not connected", msg)
	}
	if !m.Popups.IsVisible(popupctl.Export) {
		t.Error("the export popup closed on a refused export")
	}
	if n := m.ActiveJobCount(); n != 0 {
		t.Errorf("%d jobs in the job bar for a refused export", n)
	}
}
