package downloads

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/llehouerou/waves/internal/slskd"
)

// fakeSlskd serves the transfers endpoints the Manager uses and records what
// it was asked to queue and drop.
type fakeSlskd struct {
	mu        sync.Mutex
	transfers []slskd.DownloadFile
	refuse    bool
	queuedFor string
	queued    []slskd.File
	dropped   []string // "user/id?query"
}

func newFakeSlskd(t *testing.T, transfers ...slskd.DownloadFile) (*fakeSlskd, *slskd.Client) {
	t.Helper()
	f := &fakeSlskd{transfers: transfers}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v0/transfers/downloads", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var resp []slskd.DownloadsResponse // one per user, in first-seen order
		for _, tr := range f.transfers {
			i := len(resp) - 1
			if i < 0 || resp[i].Username != tr.Username {
				resp = append(resp, slskd.DownloadsResponse{Username: tr.Username, Directories: []slskd.DownloadDirectory{{}}})
				i++
			}
			resp[i].Directories[0].Files = append(resp[i].Directories[0].Files, tr)
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("POST /api/v0/transfers/downloads/{user}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.refuse {
			http.Error(w, "refused", http.StatusInternalServerError)
			return
		}
		f.queuedFor = r.PathValue("user")
		_ = json.NewDecoder(r.Body).Decode(&f.queued)
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("DELETE /api/v0/transfers/downloads/{user}/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.dropped = append(f.dropped, r.PathValue("user")+"/"+r.PathValue("id")+"?"+r.URL.RawQuery)
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, slskd.NewClient(srv.URL, "key")
}

// remote is the slskd path of a file in user's "Album" folder.
func remote(user, name string) string { return `@@` + user + `\Music\Album\` + name }

func newTestManager(t *testing.T, client *slskd.Client) (m *Manager, completed string) {
	t.Helper()
	db := setupTestDB(t)
	t.Cleanup(func() { db.Close() })
	completed = t.TempDir()
	return New(db, client, completed), completed
}

func addDownload(t *testing.T, m *Manager, user string, files ...string) int64 {
	t.Helper()
	d := Download{
		MBReleaseGroupID: "rg-" + user, MBArtistName: "Artist", MBAlbumTitle: "Album",
		SlskdUsername: user, SlskdDirectory: `@@` + user + `\Music\Album`,
	}
	for _, f := range files {
		d.Files = append(d.Files, DownloadFile{Filename: remote(user, f), Size: 3})
	}
	id, err := m.create(d)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// writeCompleted puts a finished 3-byte file where slskd would.
func writeCompleted(t *testing.T, completed, name string) string {
	t.Helper()
	path := filepath.Join(completed, "Album", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSync_ReadsTransfersThenVerifiesOnDisk(t *testing.T) {
	_, client := newFakeSlskd(t,
		slskd.DownloadFile{ID: "t1", Username: "bob", Filename: remote("bob", "01.flac"), State: "Completed, Succeeded", BytesTransferred: 3},
		slskd.DownloadFile{ID: "t2", Username: "bob", Filename: remote("bob", "02.flac"), State: "Completed, Succeeded", BytesTransferred: 3},
	)
	m, completed := newTestManager(t, client)
	id := addDownload(t, m, "bob", "01.flac", "02.flac")
	writeCompleted(t, completed, "01.flac")

	if err := m.Sync(); err != nil {
		t.Fatal(err)
	}

	d, _ := m.Get(id)
	if d.Status != StatusCompleted {
		t.Errorf("status = %q, want completed", d.Status)
	}
	if !d.Files[0].VerifiedOnDisk || d.Files[1].VerifiedOnDisk {
		t.Errorf("verified = %v/%v, want true/false", d.Files[0].VerifiedOnDisk, d.Files[1].VerifiedOnDisk)
	}
}

// Delete and FinishImport drop only the transfers backing the download: same
// user, same files.
var backingTransfers = []slskd.DownloadFile{
	{ID: "mine", Username: "bob", Filename: remote("bob", "01.flac")},
	{ID: "same-file-other-user", Username: "alice", Filename: remote("bob", "01.flac")},
	{ID: "other-file", Username: "bob", Filename: remote("bob", "other.flac")},
}

func TestDelete_DropsTransfersFilesAndRow(t *testing.T) {
	f, client := newFakeSlskd(t, backingTransfers...)
	m, completed := newTestManager(t, client)
	id := addDownload(t, m, "bob", "01.flac")
	path := writeCompleted(t, completed, "01.flac")

	if err := m.Delete(id); err != nil {
		t.Fatal(err)
	}

	if want := []string{"bob/mine?remove=true"}; !reflect.DeepEqual(f.dropped, want) {
		t.Errorf("dropped = %v, want %v", f.dropped, want)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Errorf("download folder still there: %v", err)
	}
	if _, err := os.Stat(completed); err != nil {
		t.Errorf("completed folder gone: %v", err)
	}
	if _, err := m.Get(id); err == nil {
		t.Error("row still there")
	}
}

func TestRetry_RequeuesOnlyFailedFiles(t *testing.T) {
	f, client := newFakeSlskd(t)
	m, _ := newTestManager(t, client)
	id := addDownload(t, m, "bob", "01.flac", "02.flac")
	if err := m.updateFromSlskd([]slskd.Download{
		{Username: "bob", Filename: remote("bob", "01.flac"), State: "Completed, Succeeded", BytesTransferred: 3},
		{Username: "bob", Filename: remote("bob", "02.flac"), State: "Completed, Errored"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := m.Retry(id); err != nil {
		t.Fatal(err)
	}

	want := []slskd.File{{Filename: remote("bob", "02.flac"), Size: 3}}
	if f.queuedFor != "bob" || !reflect.DeepEqual(f.queued, want) {
		t.Errorf("queued %v for %q, want %v for bob", f.queued, f.queuedFor, want)
	}
}

// Two downloads named "Album" share one folder: finishing one's import must
// leave the other's files, and remove the folder only once it is empty.
func TestFinishImport_RemovesOnlyAnEmptyFolder(t *testing.T) {
	f, client := newFakeSlskd(t, backingTransfers...)
	m, completed := newTestManager(t, client)
	bob := addDownload(t, m, "bob", "01.flac")
	alice := addDownload(t, m, "alice", "01.flac")
	alicesFile := writeCompleted(t, completed, "01.flac") // bob's was moved by the import

	if err := m.FinishImport(bob); err != nil {
		t.Fatal(err)
	}
	if want := []string{"bob/mine?remove=true"}; !reflect.DeepEqual(f.dropped, want) {
		t.Errorf("dropped = %v, want %v", f.dropped, want)
	}
	if _, err := os.Stat(alicesFile); err != nil {
		t.Errorf("another download's file removed: %v", err)
	}
	if _, err := m.Get(bob); err == nil {
		t.Error("row still there")
	}

	if err := os.Remove(alicesFile); err != nil { // alice's import moves it
		t.Fatal(err)
	}
	if err := m.FinishImport(alice); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(alicesFile)); !os.IsNotExist(err) {
		t.Errorf("empty folder kept: %v", err)
	}
	if _, err := os.Stat(completed); err != nil {
		t.Errorf("completed folder gone: %v", err)
	}
}

func TestClearCompleted_DropsOnlyCompleted(t *testing.T) {
	f, client := newFakeSlskd(t,
		slskd.DownloadFile{ID: "done", Username: "bob", Filename: remote("bob", "01.flac")},
		slskd.DownloadFile{ID: "running", Username: "alice", Filename: remote("alice", "01.flac")},
	)
	m, completed := newTestManager(t, client)
	done := addDownload(t, m, "bob", "01.flac")
	addDownload(t, m, "alice", "01.flac")
	if _, err := m.db.Exec(`UPDATE downloads SET status = ? WHERE id = ?`, StatusCompleted, done); err != nil {
		t.Fatal(err)
	}
	path := writeCompleted(t, completed, "01.flac")

	if err := m.ClearCompleted(); err != nil {
		t.Fatal(err)
	}

	if want := []string{"bob/done?remove=true"}; !reflect.DeepEqual(f.dropped, want) {
		t.Errorf("dropped = %v, want %v", f.dropped, want)
	}
	left, _ := m.List()
	if len(left) != 1 || left[0].SlskdUsername != "alice" {
		t.Errorf("left = %+v, want alice's download only", left)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file removed: %v", err)
	}
}

func TestQueue_RecordsOnlyWhatSlskdAccepted(t *testing.T) {
	f, client := newFakeSlskd(t)
	m, _ := newTestManager(t, client)
	d := Download{
		MBReleaseGroupID: "rg", MBArtistName: "Artist", MBAlbumTitle: "Album",
		SlskdUsername: "bob", SlskdDirectory: `@@bob\Music\Album`,
		Files: []DownloadFile{{Filename: remote("bob", "01.flac"), Size: 3}},
	}

	f.refuse = true
	if _, err := m.Queue(d); err == nil {
		t.Fatal("Queue succeeded though slskd refused")
	}
	if all, _ := m.List(); len(all) != 0 {
		t.Fatalf("refused download recorded: %+v", all)
	}

	f.refuse = false
	id, err := m.Queue(d)
	if err != nil {
		t.Fatal(err)
	}
	if want := []slskd.File{{Filename: remote("bob", "01.flac"), Size: 3}}; f.queuedFor != "bob" || !reflect.DeepEqual(f.queued, want) {
		t.Errorf("queued %v for %q", f.queued, f.queuedFor)
	}
	if got, err := m.Get(id); err != nil || len(got.Files) != 1 {
		t.Errorf("recorded %+v, %v", got, err)
	}
}

func TestWithoutSlskd(t *testing.T) {
	m, completed := newTestManager(t, nil)
	id := addDownload(t, m, "bob", "01.flac")
	path := writeCompleted(t, completed, "01.flac")

	if err := m.Sync(); err != nil {
		t.Errorf("Sync = %v, want nothing to do", err)
	}
	if err := m.Retry(id); !errors.Is(err, ErrNoSlskd) {
		t.Errorf("Retry = %v, want ErrNoSlskd", err)
	}
	if _, err := m.Queue(Download{}); !errors.Is(err, ErrNoSlskd) {
		t.Errorf("Queue = %v, want ErrNoSlskd", err)
	}
	if err := m.Delete(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still there: %v", err)
	}
	if _, err := m.Get(id); err == nil {
		t.Error("row still there")
	}
}

// A download without a folder of its own must never take the completed folder
// (or anything above it) with it.
func TestDelete_NeverRemovesAboveItsFolder(t *testing.T) {
	for _, dir := range []string{"", `@@bob\..`} {
		m, completed := newTestManager(t, nil)
		id, err := m.create(Download{MBReleaseGroupID: "rg", MBArtistName: "A", MBAlbumTitle: "B", SlskdUsername: "bob", SlskdDirectory: dir})
		if err != nil {
			t.Fatal(err)
		}
		keep := writeCompleted(t, completed, "other.flac")

		if err := m.Delete(id); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("dir %q: removed %s: %v", dir, keep, err)
		}
	}
}

// A folder that can't be removed keeps the row, so the user can delete again.
func TestDelete_KeepsRowWhenFilesCannotBeRemoved(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	m, completed := newTestManager(t, nil)
	id := addDownload(t, m, "bob", "01.flac")
	path := writeCompleted(t, completed, "01.flac")
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if err := m.Delete(id); err == nil {
		t.Fatal("Delete succeeded though the files are still there")
	}
	if _, err := m.Get(id); err != nil {
		t.Errorf("row dropped: %v", err)
	}
}

// slskd being unreachable must not block removal, but a Sync against it fails.
func TestSlskdDown(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	m, completed := newTestManager(t, slskd.NewClient(srv.URL, "key"))
	id := addDownload(t, m, "bob", "01.flac")
	path := writeCompleted(t, completed, "01.flac")

	if err := m.Sync(); err == nil {
		t.Error("Sync succeeded with slskd down")
	}
	if err := m.Delete(id); err != nil {
		t.Errorf("Delete = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still there: %v", err)
	}
	if left, _ := m.List(); len(left) != 0 {
		t.Errorf("left = %+v, want none", left)
	}
}
