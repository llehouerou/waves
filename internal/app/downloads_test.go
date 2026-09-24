package app

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/errmsg"
	dlview "github.com/llehouerou/waves/internal/ui/downloads"
)

func TestDownloadsChanged(t *testing.T) {
	list := []downloads.Download{{ID: 7, MBAlbumTitle: "Album"}}
	tests := []struct {
		name      string
		msg       DownloadsChangedMsg
		wantList  int
		wantError string
	}{
		{"operation succeeded", DownloadsChangedMsg{Op: errmsg.OpDownloadDelete, Downloads: list}, 1, ""},
		{"operation failed, list read", DownloadsChangedMsg{Op: errmsg.OpDownloadDelete, Downloads: list, Err: errors.New("boom")}, 1, "delete download"},
		{"sync failed: silent", DownloadsChangedMsg{Op: errmsg.OpDownloadRefresh, Downloads: list, Err: errors.New("down")}, 1, ""},
		{"list unreadable: view kept", DownloadsChangedMsg{Op: errmsg.OpDownloadClear, Err: errors.New("db")}, 2, "clear completed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel()
			m.DownloadsView = dlview.New()
			m.DownloadsView.SetDownloads([]downloads.Download{{ID: 1}, {ID: 2}})

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

// drain runs cmd and every command it batches or sequences, and returns the
// messages they produce. blocked reports a command still running after a
// second: a timer, such as the polling loop's next tick.
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

// Only Init starts the polling loop. Opening the downloads view (from a key
// or at startup) used to start one more each time.
func TestOpeningDownloadsView_StartsNoPollingLoop(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := newTestModel()
	m.Downloads = downloads.New(db, nil, "") // no tables: List fails, which is fine here

	msgs, blocked := drain(t, m.loadAndRefreshDownloads())

	if blocked {
		t.Error("opening the view scheduled a timer")
	}
	var changed int
	for _, msg := range msgs {
		switch msg.(type) {
		case DownloadsRefreshMsg:
			t.Errorf("opening the view sent %T, starting another polling loop", msg)
		case DownloadsChangedMsg:
			changed++
		}
	}
	if changed != 2 {
		t.Errorf("got %d list results, want 2 (load, then sync): %#v", changed, msgs)
	}
}
