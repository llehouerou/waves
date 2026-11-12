package core

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/slskd/slskdtest"
)

// newDownloadsCore builds a core on a fake slskd, with a completed folder of
// its own.
func newDownloadsCore(t *testing.T, opts Options) (c *Core, fake *slskdtest.Server, completed string) {
	t.Helper()
	fake = slskdtest.New(t)
	completed = t.TempDir()
	opts.Slskd = config.SlskdConfig{URL: fake.URL, APIKey: "key", CompletedPath: completed}
	c, _ = newTestCore(t, opts)
	return c, fake, completed
}

// remote is the slskd path of a file in bob's "Album" folder.
func remote(name string) string { return `@@bob\Music\Album\` + name }

// album is a download of bob's "Album" folder.
func album(files ...string) Download {
	d := Download{
		MBReleaseGroupID: "rg", MBArtistName: "Low", MBAlbumTitle: "Trust",
		SlskdUsername: "bob", SlskdDirectory: `@@bob\Music\Album`,
	}
	for _, f := range files {
		d.Files = append(d.Files, DownloadFile{Filename: remote(f), Size: 3})
	}
	return d
}

// transfer is slskd's record of one of bob's files, in the given state.
func transfer(name, state string) slskd.DownloadFile {
	return slskd.DownloadFile{ID: "t-" + name, Username: "bob", Filename: remote(name), State: state, BytesTransferred: 3}
}

// writeCompleted puts a finished file where slskd writes bob's "Album".
func writeCompleted(t *testing.T, completed, name string) {
	t.Helper()
	path := filepath.Join(completed, "Album", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create download folder: %v", err)
	}
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestSync_FollowsAQueuedDownloadToCompletedWithNoClient(t *testing.T) {
	c, fake, completed := newDownloadsCore(t, Options{})
	dl := c.Downloads()

	id, err := dl.Queue(album("01.flac", "02.flac"))
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	// slskd finishes the transfers and writes the files.
	fake.SetTransfers(transfer("01.flac", "Completed, Succeeded"), transfer("02.flac", "Completed, Succeeded"))
	writeCompleted(t, completed, "01.flac")
	writeCompleted(t, completed, "02.flac")

	// Not an event is read, nothing more is asked: the core syncs on its own
	// clock.
	eventually(t, "the download is completed and verified on disk", func() bool {
		all, err := dl.List()
		if err != nil || len(all) != 1 || all[0].ID != id || len(all[0].Files) != 2 {
			return false
		}
		d := all[0]
		return d.Status == DownloadCompleted && d.Files[0].VerifiedOnDisk && d.Files[1].VerifiedOnDisk
	})
}

func TestSync_ReportsTheDownloadsOnlyWhenTheyChange(t *testing.T) {
	c, fake, _ := newDownloadsCore(t, Options{})
	if _, err := c.Downloads().Queue(album("01.flac")); err != nil {
		t.Fatalf("queue: %v", err)
	}

	if e := nextEvent[DownloadsChanged](t, c); len(e.Downloads) != 1 || e.Downloads[0].Status != DownloadPending {
		t.Fatalf("DownloadsChanged = %+v, want the download just queued, pending", e.Downloads)
	}

	fake.SetTransfers(transfer("01.flac", "InProgress"))
	if e := nextEvent[DownloadsChanged](t, c); len(e.Downloads) != 1 || e.Downloads[0].Status != DownloadDownloading {
		t.Fatalf("DownloadsChanged = %+v, want the download downloading", e.Downloads)
	}

	// Several passes later slskd says the same: there is nothing to report.
	expectNoEvent[DownloadsChanged](t, c)
}

func TestSync_ReportsAnOutageOfSlskdOnceAndGoesOn(t *testing.T) {
	c, fake, _ := newDownloadsCore(t, Options{})

	fake.Down(true)
	if e := waitFailed(t, c, errmsg.OpDownloadRefresh); e.Err == nil {
		t.Error("Failed carries no error")
	}
	// Several failed passes later: the outage was said once.
	expectNoEvent[Failed](t, c)

	fake.Down(false)
	if _, err := c.Downloads().Queue(album("01.flac")); err != nil {
		t.Fatalf("queue: %v", err)
	}
	nextEvent[DownloadsChanged](t, c)

	// Another outage is another report.
	fake.Down(true)
	waitFailed(t, c, errmsg.OpDownloadRefresh)
}

func TestDownloads_WithoutSlskdNothingIsQueuedNorSynced(t *testing.T) {
	c, _ := newTestCore(t, Options{})
	dl := c.Downloads()

	if dl.Configured() {
		t.Error("Configured() = true with no slskd in the options")
	}
	if _, err := dl.Queue(album("01.flac")); !errors.Is(err, downloads.ErrNoSlskd) {
		t.Errorf("Queue = %v, want ErrNoSlskd", err)
	}
	dl.Sync() // asks for nothing: there is no slskd to ask
	if all, err := dl.List(); err != nil || len(all) != 0 {
		t.Errorf("List = %+v (err %v), want no download", all, err)
	}
	expectNoEvent[Failed](t, c)
}

func TestClose_DoesNotWaitForAnSlskdThatDoesNotAnswer(t *testing.T) {
	asked := make(chan struct{}, 1)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case asked <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})
	c, _ := newTestCore(t, Options{Slskd: config.SlskdConfig{URL: srv.URL, APIKey: "key"}})

	select {
	case <-asked: // a Sync is waiting for slskd
	case <-time.After(5 * time.Second):
		t.Fatal("no Sync reached slskd")
	}

	closed := make(chan error, 1)
	go func() { closed <- c.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Errorf("close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close is still waiting for slskd")
	}

	// The Sync it interrupted is not an outage of slskd.
	for e := range c.Events() {
		if failed, ok := e.(Failed); ok {
			t.Errorf("Close reported %+v", failed)
		}
	}
}

func TestQueueAndSync_RunAPassWithoutWaitingForTheClock(t *testing.T) {
	// A clock that never ticks while the test runs.
	c, fake, _ := newDownloadsCore(t, Options{tick: time.Hour})
	dl := c.Downloads()

	if _, err := dl.Queue(album("01.flac")); err != nil {
		t.Fatalf("queue: %v", err)
	}
	if e := nextEvent[DownloadsChanged](t, c); len(e.Downloads) != 1 {
		t.Fatalf("DownloadsChanged = %+v, want the download just queued", e.Downloads)
	}

	fake.SetTransfers(transfer("01.flac", "InProgress"))
	dl.Sync()
	if e := nextEvent[DownloadsChanged](t, c); len(e.Downloads) != 1 || e.Downloads[0].Status != DownloadDownloading {
		t.Fatalf("DownloadsChanged = %+v, want the download downloading", e.Downloads)
	}
}
