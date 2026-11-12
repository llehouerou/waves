package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/slskd/slskdtest"
	"github.com/llehouerou/waves/internal/ui/download"
	dlview "github.com/llehouerou/waves/internal/ui/downloads"
)

// newDownloadsTestModel builds a model whose core downloads from a fake
// slskd.
func newDownloadsTestModel(t *testing.T) (Model, *slskdtest.Server) {
	t.Helper()
	fake := slskdtest.New(t)
	m := newDatabaseTestModel(t, nil, config.SlskdConfig{URL: fake.URL, APIKey: "key", CompletedPath: t.TempDir()})
	return m, fake
}

// bobsAlbum is a download of one file of bob's "Album" folder.
func bobsAlbum() core.Download {
	return core.Download{
		MBReleaseGroupID: "rg", MBArtistName: "Low", MBAlbumTitle: "Trust",
		SlskdUsername: "bob", SlskdDirectory: `@@bob\Music\Album`,
		Files: []core.DownloadFile{{Filename: `@@bob\Music\Album\01.flac`, Size: 3}},
	}
}

// The core syncs the downloads and says when they change: the TUI has no
// polling loop of its own any more.
func TestDownloads_TheCoreSyncsThemAndTheViewFollows(t *testing.T) {
	m, fake := newDownloadsTestModel(t)

	// What the download popup does when the user picks a result.
	if _, err := m.Core.Downloads().Queue(bobsAlbum()); err != nil {
		t.Fatalf("queue: %v", err)
	}
	m = pumpUntil[ServiceDownloadsChangedMsg](t, m)
	if got := m.DownloadsView.Downloads(); len(got) != 1 || got[0].Status != core.DownloadPending {
		t.Fatalf("downloads view = %+v, want the download just queued, pending", got)
	}

	// slskd starts the transfer; "r" in the downloads view asks for a Sync
	// rather than wait for the core's clock, which ticks every three seconds.
	fake.SetTransfers(slskd.DownloadFile{
		ID: "t1", Username: "bob", Filename: `@@bob\Music\Album\01.flac`, State: "InProgress", BytesTransferred: 1,
	})
	asked := time.Now()
	m, _ = updateModel(t, m, dlview.ActionMsg(dlview.RefreshRequest{}))
	m = pumpUntil[ServiceDownloadsChangedMsg](t, m)

	if got := m.DownloadsView.Downloads(); len(got) != 1 || got[0].Status != core.DownloadDownloading {
		t.Errorf("downloads view = %+v, want the download downloading", got)
	}
	if waited := time.Since(asked); waited > 2*time.Second {
		t.Errorf("the view waited %v for the core's clock: the refresh asked for no Sync", waited)
	}
}

func TestDownloadsChanged(t *testing.T) {
	list := []core.Download{{ID: 7, MBAlbumTitle: "Album"}}
	tests := []struct {
		name      string
		msg       DownloadsChangedMsg
		wantList  int
		wantError string
	}{
		{"operation succeeded", DownloadsChangedMsg{Op: errmsg.OpDownloadDelete, Downloads: list}, 1, ""},
		{"operation failed, list read", DownloadsChangedMsg{Op: errmsg.OpDownloadDelete, Downloads: list, Err: errors.New("boom")}, 1, "delete download"},
		{"list unreadable: view kept", DownloadsChangedMsg{Op: errmsg.OpDownloadClear, Err: errors.New("db")}, 2, "clear completed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			m.DownloadsView = dlview.New()
			m.DownloadsView.SetDownloads([]core.Download{{ID: 1}, {ID: 2}})

			res, _ := m.handleDownloadMsgCategory(tt.msg)
			got, ok := res.(Model)
			if !ok {
				t.Fatalf("model is %T", res)
			}

			if n := len(got.DownloadsView.Downloads()); n != tt.wantList {
				t.Errorf("view shows %d downloads, want %d", n, tt.wantList)
			}
			if msg := got.Popups.ErrorMsg(); (tt.wantError == "") != (msg == "") || !strings.Contains(msg, tt.wantError) {
				t.Errorf("error = %q, want one mentioning %q", msg, tt.wantError)
			}
		})
	}
}

// The core reports an outage of slskd once; the TUI keeps quiet about it, as
// it did about each failed poll, and goes on watching the core.
func TestSyncFailure_IsNotShown(t *testing.T) {
	m := newIntegrationTestModel()

	res, cmd := m.Update(ServiceFailedMsg{Op: errmsg.OpDownloadRefresh, Err: errors.New("connection refused")})

	got, ok := res.(Model)
	if !ok {
		t.Fatalf("model is %T", res)
	}
	if msg := got.Popups.ErrorMsg(); msg != "" {
		t.Errorf("error popup = %q, want none", msg)
	}
	if cmd == nil {
		t.Error("the model stopped watching the core")
	}
}

// The events of a slskd search are for the download popup: with none open
// they are dropped, and the model goes on watching the core.
func TestSearchEvents_WithNoDownloadPopupAreDropped(t *testing.T) {
	m := newIntegrationTestModel()

	for _, event := range []core.Event{
		core.SearchProgressed{Query: "low trust", Phase: core.SearchSearching},
		core.SearchFinished{Query: "low trust", Err: errors.New("boom")},
	} {
		res, cmd := m.Update(ServiceSearchMsg{Event: event})
		got, ok := res.(Model)
		if !ok {
			t.Fatalf("model is %T", res)
		}
		if msg := got.Popups.ErrorMsg(); msg != "" {
			t.Errorf("%T: error popup = %q, want none", event, msg)
		}
		if cmd == nil {
			t.Errorf("%T: the model stopped watching the core", event)
		}
	}
}

// A refused queue, such as a download folder already taken, reaches the user
// as an error popup.
func TestDownloadQueueFailed_ShowsErrorPopup(t *testing.T) {
	m := newTestModel()
	err := errors.New("download folder /done/Album already exists")

	res, _ := m.Update(download.ActionMsg(download.QueueFailed{Err: err}))

	got, ok := res.(Model)
	if !ok {
		t.Fatalf("model is %T", res)
	}
	if want := errmsg.Format(errmsg.OpDownloadQueue, err); got.Popups.ErrorMsg() != want {
		t.Errorf("error = %q, want %q", got.Popups.ErrorMsg(), want)
	}
}

// drain runs cmd and every command it batches or sequences, and returns the
// messages they produce. blocked reports a command still running after a
// second: a timer.
func drain(t *testing.T, cmd tea.Cmd) (msgs []tea.Msg, blocked bool) {
	t.Helper()
	if cmd == nil {
		return nil, false
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(time.Second):
		return nil, true
	}
	var sub []tea.Cmd
	switch v := msg.(type) {
	case tea.BatchMsg:
		sub = v
	case nil:
	default:
		// tea.Sequence's message type is unexported; reflect on its shape.
		rv := reflect.ValueOf(msg)
		if rv.Kind() != reflect.Slice || rv.Type().Elem() != reflect.TypeFor[tea.Cmd]() {
			return []tea.Msg{msg}, false
		}
		for i := range rv.Len() {
			sub = append(sub, rv.Index(i).Interface().(tea.Cmd)) //nolint:forcetypeassert // element type checked above
		}
	}
	for _, c := range sub {
		m, b := drain(t, c)
		msgs, blocked = append(msgs, m...), blocked || b
	}
	return msgs, blocked
}

// Opening the downloads view (from a key or at startup) reads the downloads
// once and asks the core for a Sync: it starts no timer. It used to start one
// more polling loop each time.
func TestOpeningDownloadsView_ReadsTheListAndAsksForASync(t *testing.T) {
	m, fake := newDownloadsTestModel(t)
	if _, err := m.Core.Downloads().Queue(bobsAlbum()); err != nil {
		t.Fatalf("queue: %v", err)
	}
	m = pumpUntil[ServiceDownloadsChangedMsg](t, m)
	fake.SetTransfers(slskd.DownloadFile{
		ID: "t1", Username: "bob", Filename: `@@bob\Music\Album\01.flac`, State: "InProgress", BytesTransferred: 1,
	})

	opened := time.Now()
	msgs, blocked := drain(t, m.loadAndRefreshDownloads())

	if blocked {
		t.Error("opening the view scheduled a timer")
	}
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want the list read once: %#v", len(msgs), msgs)
	}
	if list, ok := msgs[0].(DownloadsChangedMsg); !ok || len(list.Downloads) != 1 {
		t.Errorf("message = %#v, want the one download", msgs[0])
	}

	// The Sync it asked for, not the core's clock, brings the new state.
	m = pumpUntil[ServiceDownloadsChangedMsg](t, m)
	if got := m.DownloadsView.Downloads(); len(got) != 1 || got[0].Status != core.DownloadDownloading {
		t.Errorf("downloads view = %+v, want the download downloading", got)
	}
	if waited := time.Since(opened); waited > 2*time.Second {
		t.Errorf("the view waited %v for the core's clock: opening it asked for no Sync", waited)
	}
}
