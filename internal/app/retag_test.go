package app

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/tags/tagstest"
	"github.com/llehouerou/waves/internal/ui/retag"
)

// newRetagTestModel builds a model whose library holds one album, "trust" by
// "low", of one file, shown in the library's album view.
func newRetagTestModel(t *testing.T) Model {
	t.Helper()
	source := t.TempDir()
	path := filepath.Join(source, "low", "trust", "canada.mp3")
	tagstest.WriteMP3(t, path, core.FileTags{Artist: "low", AlbumArtist: "low", Album: "trust", Title: "canada", TrackNumber: 1})
	m := newLibraryTestModel(t, func(*sql.DB) {})
	if err := m.Core.Library().AddSource(source); err != nil {
		t.Fatalf("add source: %v", err)
	}
	m = pumpUntil[LibraryScanCompleteMsg](t, m)
	m.Navigation.SetViewMode(navctl.ViewLibrary)
	m.Navigation.SetLibrarySubMode(navctl.LibraryModeAlbum)
	if err := m.Navigation.AlbumView().Refresh(); err != nil {
		t.Fatalf("album view: %v", err)
	}
	return m
}

// asLowTrust is what the retag popup asks the core once the user confirmed.
func asLowTrust() core.RetagRequest {
	return core.RetagRequest{
		AlbumArtist: "low", Album: "trust",
		Release: &core.MBReleaseDetails{
			Release: core.MBRelease{ID: "release", Title: "Trust", Artist: "Low"},
			Tracks:  []core.MBTrack{{Position: 1, Title: "Canada"}},
		},
		ReleaseGroup: &core.MBReleaseGroup{ID: "rg", PrimaryType: "Album"},
	}
}

// The user closed the popup while the core retagged: when it is done the
// library views show the album under the names the retag wrote, with nothing
// asked of the popup.
func TestRetag_TheCoreRunsItAndTheLibraryViewsFollow(t *testing.T) {
	m := newRetagTestModel(t)
	if album := m.Navigation.AlbumView().SelectedAlbum(); album == nil || album.Album != "trust" {
		t.Fatalf("selected album = %+v, want trust before the retag", album)
	}

	if err := m.Core.Library().Retag(asLowTrust()); err != nil {
		t.Fatalf("retag: %v", err)
	}
	m = pumpUntilMatch(t, m, func(msg tea.Msg) bool {
		retagMsg, ok := msg.(ServiceRetagMsg)
		if !ok {
			return false
		}
		_, finished := retagMsg.Event.(core.RetagFinished)
		return finished
	})

	if album := m.Navigation.AlbumView().SelectedAlbum(); album == nil || album.AlbumArtist != "Low" || album.Album != "Trust" {
		t.Errorf("album view shows %+v, want Low - Trust", album)
	}
	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want none", msg)
	}
}

// With no popup left to show it, a failure is an error popup.
func TestRetagFinished_WithThePopupClosedShowsAFailure(t *testing.T) {
	failed := []core.FileProgress{{Path: "/music/low/trust/canada.mp3", State: core.FileFailed, Err: errors.New("read-only file")}}
	tests := []struct {
		name     string
		finished core.RetagFinished
		want     string
	}{
		{"a file failed", core.RetagFinished{AlbumArtist: "low", Album: "trust", Files: failed},
			"Retag of low - trust: 1 of 1 files failed"},
		{"the library was not updated", core.RetagFinished{AlbumArtist: "low", Album: "trust", Err: errors.New("database is locked")},
			"Retag of low - trust: library not updated: database is locked"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newRetagTestModel(t)

			m.handleRetagEvent(tt.finished)

			if got := m.Popups.ErrorMsg(); got != tt.want {
				t.Errorf("error popup = %q, want %q", got, tt.want)
			}
		})
	}
}

// The popup shows its own failures, and closes itself when there is none.
func TestRetagFinished_WithThePopupOpenLeavesItToThePopup(t *testing.T) {
	m := newRetagTestModel(t)
	m.Popups.ShowRetag("low", "trust", m.Core.MusicBrainz(), m.Core.Library())
	failed := []core.FileProgress{{Path: "/music/low/trust/canada.mp3", State: core.FileFailed, Err: errors.New("read-only file")}}

	m.handleRetagEvent(core.RetagFinished{AlbumArtist: "low", Album: "trust", Files: failed})

	if msg := m.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want the failure left to the retag popup", msg)
	}
	if !m.Popups.IsVisible(popupctl.Retag) {
		t.Error("the retag popup was closed on a failure")
	}
}

// The popup is done with its retag: the model closes it. Stopping playback of
// a track being retagged is no longer asked of the model: the core does it.
func TestRetagComplete_ClosesThePopup(t *testing.T) {
	m := newRetagTestModel(t)
	m.Popups.ShowRetag("low", "trust", m.Core.MusicBrainz(), m.Core.Library())

	m, _ = updateModel(t, m, retag.ActionMsg(retag.Complete{AlbumArtist: "low", AlbumName: "trust"}))

	if m.Popups.IsVisible(popupctl.Retag) {
		t.Error("the retag popup is still open")
	}
}
