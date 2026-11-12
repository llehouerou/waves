package core

import (
	"reflect"
	"time"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/slskd"
)

// downloadsSyncTicks is how many ticks of the core's clock separate two Syncs.
const downloadsSyncTicks = 3

// The downloads data a client handles, so that it imports the core only.
type (
	Download     = downloads.Download
	DownloadFile = downloads.DownloadFile
)

// The status of a download, and of each of its files.
const (
	DownloadPending     = downloads.StatusPending
	DownloadDownloading = downloads.StatusDownloading
	DownloadCompleted   = downloads.StatusCompleted
	DownloadFailed      = downloads.StatusFailed
)

// downloadsAPI is the part of downloads.Manager a client calls as it is.
type downloadsAPI interface {
	List() ([]Download, error)
	ActiveReleaseGroups() (map[string]bool, error)

	Retry(id int64) error
	Delete(id int64) error
	ClearCompleted() error

	// FinishImport stays a command of the client until the import moves into
	// the core (issue #97).
	FinishImport(id int64) error
}

// Downloads is the downloads domain of the core: the downloads and what is
// asked of slskd about them. The core adds the Sync, which it runs on its own
// clock and reports through DownloadsChanged.
type Downloads struct {
	downloadsAPI
	mgr    *downloads.Manager
	client *slskd.Client // nil when slskd isn't configured
	core   *Core
	asked  chan struct{} // a Sync is asked for now
	done   chan struct{} // closed once the Sync loop has ended
}

// Configured reports whether there is a slskd to download from. Without one
// the downloads already recorded are kept, and can only be deleted.
func (d *Downloads) Configured() bool {
	return d.client != nil
}

// Queue queues a download on slskd, records it, and asks for a Sync.
func (d *Downloads) Queue(dl Download) (id int64, err error) {
	id, err = d.mgr.Queue(dl)
	if err == nil {
		d.Sync()
	}
	return id, err
}

// Sync asks for a Sync now rather than on the core's next tick, without
// waiting for it: DownloadsChanged follows if it changed anything.
func (d *Downloads) Sync() {
	mark(d.asked)
}

// start runs the Sync loop, when there is a slskd to sync with.
func (d *Downloads) start() {
	if d.client == nil || d.mgr == nil {
		close(d.done)
		return
	}
	go d.run()
}

// close ends what is being asked of slskd and waits for the Sync loop, which
// the core has already told to stop: it must not find the database closed.
func (d *Downloads) close() {
	if d.client != nil {
		d.client.Close()
	}
	<-d.done
}

// run syncs the downloads with slskd for as long as the core lives. One
// goroutine runs every pass: a Sync never overlaps another, however slow
// slskd is.
func (d *Downloads) run() {
	defer close(d.done)
	clock := time.NewTicker(downloadsSyncTicks * d.core.tick)
	defer clock.Stop()

	reported := []Download{} // what the client was last told
	failing := false         // the last pass failed: said once per outage
	for {
		select {
		case <-d.core.stop:
			return
		case <-clock.C:
		case <-d.asked:
		}
		current, err := d.pass()
		select {
		case <-d.core.stop:
			return // Close cut the pass short: its error is not an outage
		default:
		}
		if err != nil {
			// A pass every few seconds: reporting each failure of an slskd
			// that is down would fill the event stream.
			if !failing {
				d.core.emit(Failed{Op: errmsg.OpDownloadRefresh, Err: err})
			}
			failing = true
			continue
		}
		failing = false
		if !reflect.DeepEqual(current, reported) {
			reported = current
			d.core.emit(DownloadsChanged{Downloads: current})
		}
	}
}

// pass is one Sync, and the downloads as it leaves them.
func (d *Downloads) pass() ([]Download, error) {
	if err := d.mgr.Sync(); err != nil {
		return nil, err
	}
	return d.mgr.List()
}
