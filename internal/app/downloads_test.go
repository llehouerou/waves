package app

import (
	"errors"
	"strings"
	"testing"

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
