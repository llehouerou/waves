package download

import (
	"errors"
	"reflect"
	"testing"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/ui/action"
)

// The highlighted slskd result is recorded against the selected release.
func TestSelectedDownload(t *testing.T) {
	m := New(nil, nil, FilterConfig{}, nil)
	m.state = StateSlskdResults
	rg := &musicbrainz.ReleaseGroup{ID: "rg", Title: "Album", FirstRelease: "1999"}
	m.selectedArtist = &musicbrainz.Artist{Name: "Artist"}
	m.selectedReleaseGroup = rg
	m.selectedRelease = &musicbrainz.Release{ID: "rel"}
	details := &musicbrainz.ReleaseDetails{ReleaseGroupID: "rg"}
	m.selectedReleaseDetails = details
	m.slskdResults = []SlskdResult{
		{Username: "alice", Directory: `@@alice\A`},
		{Username: "bob", Directory: `@@bob\B`, Files: []slskd.File{{Filename: `@@bob\B\01.flac`, Size: 3}}},
	}
	m.slskdCursor.SetPos(1)

	got, ok := m.selectedDownload()

	want := downloads.Download{
		MBReleaseGroupID: "rg", MBReleaseID: "rel", MBArtistName: "Artist",
		MBAlbumTitle: "Album", MBReleaseYear: "1999", MBReleaseGroup: rg, MBReleaseDetails: details,
		SlskdUsername: "bob", SlskdDirectory: `@@bob\B`,
		Files: []downloads.DownloadFile{{Filename: `@@bob\B\01.flac`, Size: 3}},
	}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("selectedDownload() = %+v, %v\nwant %+v", got, ok, want)
	}

	// Enter queues exactly that download.
	if cmd := m.handleSlskdEnter(); cmd == nil || m.state != StateDownloading {
		t.Errorf("Enter: cmd %v, state %d; want a queue command and StateDownloading", cmd != nil, m.state)
	}

	for name, clear := range map[string]func(){
		"artist":        func() { m.selectedArtist = nil },
		"release group": func() { m.selectedReleaseGroup = nil },
	} {
		clear()
		if _, ok := m.selectedDownload(); ok {
			t.Errorf("recorded a download without a %s", name)
		}
	}
	m.state = StateSlskdResults
	if cmd := m.handleSlskdEnter(); cmd != nil {
		t.Error("Enter queued a download it can't record")
	}
}

// A refused queue keeps the user on the results and hands the error to the
// app; an accepted one tells the app, which syncs the recorded download.
func TestDownloadQueued(t *testing.T) {
	m := New(nil, nil, FilterConfig{}, nil)
	m.state = StateDownloading

	refused := errors.New("refused")
	_, cmd := m.handleDownloadQueued(SlskdDownloadQueuedMsg{Err: refused})
	if m.state != StateSlskdResults || cmd == nil {
		t.Fatalf("after refusal: state %d, cmd %v", m.state, cmd != nil)
	}
	if msg, ok := cmd().(action.Msg); !ok || msg.Action != (QueueFailed{Err: refused}) {
		t.Errorf("action = %#v, want QueueFailed", cmd())
	}

	_, cmd = m.handleDownloadQueued(SlskdDownloadQueuedMsg{})
	if cmd == nil {
		t.Fatal("no action after queueing")
	}
	if msg, ok := cmd().(action.Msg); !ok || msg.Action != (Queued{}) {
		t.Errorf("action = %#v, want Queued", cmd())
	}
	if !m.IsDownloadComplete() {
		t.Error("popup not marked complete")
	}
}

// From the releases list, the popup goes back to it to queue more, with the
// release marked as queued.
func TestDownloadQueued_FromReleases(t *testing.T) {
	m := New(nil, nil, FilterConfig{}, nil)
	m.fromReleases = true
	m.selectedReleaseGroup = &musicbrainz.ReleaseGroup{ID: "rg"}
	m.state = StateDownloading

	_, cmd := m.handleDownloadQueued(SlskdDownloadQueuedMsg{})

	if cmd == nil {
		t.Fatal("no action after queueing")
	}
	if msg, ok := cmd().(action.Msg); !ok || msg.Action != (Queued{}) {
		t.Errorf("action = %#v, want Queued", cmd())
	}
	if !m.relQueued["rg"] || m.IsDownloadComplete() {
		t.Errorf("relQueued = %v, complete = %v; want marked, not complete", m.relQueued, m.IsDownloadComplete())
	}
}
