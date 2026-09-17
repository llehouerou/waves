package slskd

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The bug this guards: the filename was passed where slskd expects the
// transfer GUID, so the DELETE never matched anything.
func TestCancelDownload_UsesTransferIDAndRemoves(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	transfers := []Download{
		{ID: "abc-123", Username: "bob", Filename: `@@dir\song.mp3`},
		{ID: "def-456", Username: "alice", Filename: `@@dir\song.mp3`},
	}
	ids := TransferIDs(transfers, "bob", []string{`@@dir\song.mp3`, "unknown.mp3"})
	if len(ids) != 1 || ids[0] != "abc-123" {
		t.Fatalf("TransferIDs = %v, want [abc-123]", ids)
	}

	if err := NewClient(srv.URL, "key").CancelDownload("bob", ids[0], true); err != nil {
		t.Fatalf("CancelDownload: %v", err)
	}
	if want := "/api/v0/transfers/downloads/bob/abc-123"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if gotQuery != "remove=true" {
		t.Errorf("query = %q, want remove=true", gotQuery)
	}
}
