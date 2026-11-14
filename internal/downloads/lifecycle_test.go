package downloads

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/slskd/slskdtest"
)

// newFakeSlskd starts a fake slskd holding transfers, and returns it with a
// client of it.
func newFakeSlskd(t *testing.T, transfers ...slskd.DownloadFile) (*slskdtest.Server, *slskd.Client) {
	t.Helper()
	f := slskdtest.New(t, transfers...)
	client := f.Client()
	return f, client
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
	return writeDisc(t, completed, "Album", name)
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

	if want := []string{"bob/mine?remove=true"}; !reflect.DeepEqual(f.Dropped(), want) {
		t.Errorf("dropped = %v, want %v", f.Dropped(), want)
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
	if user, queued := f.Queued(); user != "bob" || !reflect.DeepEqual(queued, want) {
		t.Errorf("queued %v for %q, want %v for bob", queued, user, want)
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
	if want := []string{"bob/mine?remove=true"}; !reflect.DeepEqual(f.Dropped(), want) {
		t.Errorf("dropped = %v, want %v", f.Dropped(), want)
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

	if kept, err := m.ClearCompleted(0); err != nil || kept {
		t.Fatalf("ClearCompleted = kept %v, %v; want nothing kept, no error", kept, err)
	}

	if want := []string{"bob/done?remove=true"}; !reflect.DeepEqual(f.Dropped(), want) {
		t.Errorf("dropped = %v, want %v", f.Dropped(), want)
	}
	left, _ := m.List()
	if len(left) != 1 || left[0].SlskdUsername != "alice" {
		t.Errorf("left = %+v, want alice's download only", left)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file removed: %v", err)
	}
}

func TestClearCompleted_KeepsTheDownloadItIsTold(t *testing.T) {
	f, client := newFakeSlskd(t,
		slskd.DownloadFile{ID: "bobs", Username: "bob", Filename: remote("bob", "01.flac")},
		slskd.DownloadFile{ID: "alices", Username: "alice", Filename: remote("alice", "01.flac")},
	)
	m, _ := newTestManager(t, client)
	bob := addDownload(t, m, "bob", "01.flac")
	alice := addDownload(t, m, "alice", "01.flac")
	if _, err := m.db.Exec(`UPDATE downloads SET status = ?`, StatusCompleted); err != nil {
		t.Fatal(err)
	}

	kept, err := m.ClearCompleted(alice)
	if err != nil || !kept {
		t.Fatalf("ClearCompleted = kept %v, %v; want alice's download kept", kept, err)
	}

	if want := []string{"bob/bobs?remove=true"}; !reflect.DeepEqual(f.Dropped(), want) {
		t.Errorf("dropped = %v, want %v", f.Dropped(), want)
	}
	left, _ := m.List()
	if len(left) != 1 || left[0].ID != alice {
		t.Errorf("left = %+v, want alice's download only (bob's is %d)", left, bob)
	}
}

// A download that is not completed is not one ClearCompleted would drop: it
// is not said to be kept.
func TestClearCompleted_KeepingADownloadNotCompletedKeepsNothing(t *testing.T) {
	_, client := newFakeSlskd(t)
	m, _ := newTestManager(t, client)
	pending := addDownload(t, m, "alice", "01.flac")

	if kept, err := m.ClearCompleted(pending); err != nil || kept {
		t.Fatalf("ClearCompleted = kept %v, %v; want nothing kept, no error", kept, err)
	}
	if left, _ := m.List(); len(left) != 1 {
		t.Errorf("left = %+v, want the pending download", left)
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

	f.Refuse(true)
	if _, err := m.Queue(d); err == nil {
		t.Fatal("Queue succeeded though slskd refused")
	}
	if all, _ := m.List(); len(all) != 0 {
		t.Fatalf("refused download recorded: %+v", all)
	}

	f.Refuse(false)
	id, err := m.Queue(d)
	if err != nil {
		t.Fatal(err)
	}
	want := []slskd.File{{Filename: remote("bob", "01.flac"), Size: 3}}
	if user, queued := f.Queued(); user != "bob" || !reflect.DeepEqual(queued, want) {
		t.Errorf("queued %v for %q", queued, user)
	}
	if got, err := m.Get(id); err != nil || len(got.Files) != 1 {
		t.Errorf("recorded %+v, %v", got, err)
	}
}

// A download folder belongs to one download: queueing one whose folder a
// tracked download uses, or that is on disk with anything in it, asks nothing of
// slskd and records nothing. An empty folder on disk is not taken.
func TestQueue_RefusesATakenFolder(t *testing.T) {
	queue := func(t *testing.T, m *Manager) error {
		t.Helper()
		_, err := m.Queue(Download{
			MBReleaseGroupID: "rg", MBArtistName: "Other", MBAlbumTitle: "Other",
			SlskdUsername: "alice", SlskdDirectory: `@@alice\Rips\Album`,
			Files: []DownloadFile{{Filename: `@@alice\Rips\Album\01.flac`, Size: 3}},
		})
		return err
	}
	assertRefused := func(t *testing.T, f *slskdtest.Server, m *Manager, err error, want string, rows int) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Queue = %v, want an error naming %q", err, want)
		}
		if user, _ := f.Queued(); user != "" {
			t.Errorf("queued on slskd for %q", user)
		}
		if all, _ := m.List(); len(all) != rows {
			t.Errorf("%d rows, want %d", len(all), rows)
		}
	}

	t.Run("tracked download", func(t *testing.T) {
		f, client := newFakeSlskd(t)
		m, completed := newTestManager(t, client)
		addDownload(t, m, "bob", "01.flac")
		err := queue(t, m)
		assertRefused(t, f, m, err, filepath.Join(completed, "Album"), 1)
		assertRefused(t, f, m, err, "Artist / Album", 1)
	})

	t.Run("tracked download, no completed folder", func(t *testing.T) {
		f, client := newFakeSlskd(t)
		db := setupTestDB(t)
		t.Cleanup(func() { db.Close() })
		m := New(db, client, "")
		addDownload(t, m, "bob", "01.flac")
		assertRefused(t, f, m, queue(t, m), "Artist / Album", 1)
	})

	t.Run("folder on disk", func(t *testing.T) {
		f, client := newFakeSlskd(t)
		m, completed := newTestManager(t, client)
		writeDisc(t, completed, "Album", "cover.jpg")
		assertRefused(t, f, m, queue(t, m), filepath.Join(completed, "Album")+" already exists", 0)
	})

	t.Run("unreadable folder on disk", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores directory permissions")
		}
		f, client := newFakeSlskd(t)
		m, completed := newTestManager(t, client)
		dir := filepath.Join(completed, "Album")
		if err := os.Mkdir(dir, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		assertRefused(t, f, m, queue(t, m), dir+" already exists", 0)
	})

	// v0.1.50 left the folder of each imported album behind, empty: an
	// empty folder is not taken, and slskd writes into it.
	t.Run("empty folder on disk", func(t *testing.T) {
		f, client := newFakeSlskd(t)
		m, completed := newTestManager(t, client)
		dir := mkEmptyDisc(t, completed, "Album")
		if err := queue(t, m); err != nil {
			t.Fatalf("Queue = %v, want queued", err)
		}
		if user, files := f.Queued(); user != "alice" || len(files) != 1 {
			t.Errorf("queued %d files for %q; want 1 for alice", len(files), user)
		}
		if all, _ := m.List(); len(all) != 1 {
			t.Errorf("%d rows, want 1", len(all))
		}
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("empty folder not left in place: %v", err)
		}
	})

	t.Run("no completed folder", func(t *testing.T) {
		f, client := newFakeSlskd(t)
		db := setupTestDB(t)
		t.Cleanup(func() { db.Close() })
		m := New(db, client, "")
		if err := queue(t, m); err != nil {
			t.Fatalf("Queue = %v", err)
		}
		if user, _ := f.Queued(); user != "alice" {
			t.Errorf("queued for %q; want queued for alice", user)
		}
	})
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
	for _, file := range []string{"01.flac", `@@bob\..\01.flac`} {
		m, completed := newTestManager(t, nil)
		id, err := m.create(Download{
			MBReleaseGroupID: "rg", MBArtistName: "A", MBAlbumTitle: "B", SlskdUsername: "bob",
			Files: []DownloadFile{{Filename: file}},
		})
		if err != nil {
			t.Fatal(err)
		}
		keep := writeCompleted(t, completed, "other.flac")

		if err := m.Delete(id); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("file %q: removed %s: %v", file, keep, err)
		}
	}
}

// discDownload is a download of a release split into disc folders: slskd
// writes each disc to its own download folder, CD1 and CD2.
func discDownload() Download {
	return Download{
		MBReleaseGroupID: "rg", MBArtistName: "Artist", MBAlbumTitle: "Double",
		SlskdUsername: "bob", SlskdDirectory: `@@bob\Music\Double`,
		Files: []DownloadFile{
			{Filename: `@@bob\Music\Double\CD1\01.flac`, Size: 3},
			{Filename: `@@bob\Music\Double\CD2\01.flac`, Size: 3},
		},
	}
}

// writeDisc puts a finished file in download folder disc.
func writeDisc(t *testing.T, completed, disc, name string) string {
	t.Helper()
	path := filepath.Join(completed, disc, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// mkEmptyDisc creates download folder disc, empty.
func mkEmptyDisc(t *testing.T, completed, disc string) string {
	t.Helper()
	dir := filepath.Join(completed, disc)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDiscDownload_EveryFolderIsItsOwn(t *testing.T) {
	t.Run("delete removes every folder", func(t *testing.T) {
		m, completed := newTestManager(t, nil)
		id, err := m.create(discDownload())
		if err != nil {
			t.Fatal(err)
		}
		cd1 := writeDisc(t, completed, "CD1", "01.flac")
		cd2 := writeDisc(t, completed, "CD2", "cover.jpg")

		if err := m.Delete(id); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{cd1, cd2} {
			if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
				t.Errorf("download folder %s still there: %v", filepath.Dir(path), err)
			}
		}
	})

	t.Run("finish import removes every empty folder", func(t *testing.T) {
		m, completed := newTestManager(t, nil)
		id, err := m.create(discDownload())
		if err != nil {
			t.Fatal(err)
		}
		cd1 := filepath.Dir(writeDisc(t, completed, "CD1", "x"))
		if err := os.Remove(filepath.Join(cd1, "x")); err != nil {
			t.Fatal(err)
		}
		extra := writeDisc(t, completed, "CD2", "disc.cue")

		if err := m.FinishImport(id); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(cd1); !os.IsNotExist(err) {
			t.Errorf("empty folder kept: %v", err)
		}
		if _, err := os.Stat(extra); err != nil {
			t.Errorf("folder with extras removed: %v", err)
		}
	})

	t.Run("sync finds each file in its folder", func(t *testing.T) {
		m, completed := newTestManager(t, nil)
		id, err := m.create(discDownload())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := m.db.Exec(`UPDATE downloads SET status = ?`, StatusCompleted); err != nil {
			t.Fatal(err)
		}
		writeDisc(t, completed, "CD2", "01.flac")

		if err := m.verifyOnDisk(); err != nil {
			t.Fatal(err)
		}
		d, _ := m.Get(id)
		if d.Files[0].VerifiedOnDisk || !d.Files[1].VerifiedOnDisk {
			t.Errorf("verified CD1/CD2 = %v/%v, want false/true", d.Files[0].VerifiedOnDisk, d.Files[1].VerifiedOnDisk)
		}
	})

	t.Run("queue refuses when any folder is taken", func(t *testing.T) {
		f, client := newFakeSlskd(t)
		m, completed := newTestManager(t, client)
		taken := Download{
			MBReleaseGroupID: "rg2", MBArtistName: "Other", MBAlbumTitle: "Set",
			SlskdUsername: "alice", SlskdDirectory: `@@alice\Set`,
			Files: []DownloadFile{{Filename: `@@alice\Set\CD2\01.flac`, Size: 3}},
		}
		if _, err := m.create(taken); err != nil {
			t.Fatal(err)
		}

		_, err := m.Queue(discDownload())
		if want := filepath.Join(completed, "CD2"); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Queue = %v, want an error naming %s", err, want)
		}
		if user, _ := f.Queued(); user != "" {
			t.Errorf("queued on slskd for %q", user)
		}

		m2, completed2 := newTestManager(t, client)
		mkEmptyDisc(t, completed2, "CD1")
		writeDisc(t, completed2, "CD2", "left.flac")
		want := filepath.Join(completed2, "CD2") + " already exists"
		if _, err := m2.Queue(discDownload()); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Queue = %v, want refused for CD2 on disk", err)
		}
		if user, _ := f.Queued(); user != "" {
			t.Errorf("queued on slskd for %q", user)
		}
		if all, _ := m2.List(); len(all) != 0 {
			t.Errorf("%d rows, want 0", len(all))
		}
	})

	t.Run("queue accepts an empty disc folder", func(t *testing.T) {
		f, client := newFakeSlskd(t)
		m, completed := newTestManager(t, client)
		mkEmptyDisc(t, completed, "CD1")
		if _, err := m.Queue(discDownload()); err != nil {
			t.Fatalf("Queue = %v, want queued", err)
		}
		if user, files := f.Queued(); user != "bob" || len(files) != 2 {
			t.Errorf("queued %d files for %q; want 2 for bob", len(files), user)
		}
		if all, _ := m.List(); len(all) != 1 {
			t.Errorf("%d rows, want 1", len(all))
		}
	})
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
