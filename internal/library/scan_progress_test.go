package library_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/library"
)

const (
	tick = library.ProcessingTick
	// settle is long enough for the scan to end its processing phase once
	// its last file is read or refused.
	settle = 300 * time.Millisecond
)

// heldScan is a scan of a source holding one file, whose reader stops
// reading after the first processing report while stop holds the worker
// before that file: the progress reporter is waiting in a send when the
// processing phase ends.
type heldScan struct {
	// stalled is closed once the reader stopped reading.
	stalled chan struct{}
	// released is closed once the worker is let go, past the next tick.
	released chan struct{}
	// halt is what stop answers for the file and after the phase.
	halt  bool
	calls atomic.Int64
}

func newHeldScan(halt bool) *heldScan {
	return &heldScan{stalled: make(chan struct{}), released: make(chan struct{}), halt: halt}
}

// stop answers the walk of the source (the source, then its file), then
// holds the worker before the file until the reporter, its tick unread,
// waits in its next send. A hold that sees no tick is not in the processing
// phase: it lets the scan go on, and run fails.
func (h *heldScan) stop() bool {
	switch h.calls.Add(1) {
	case 1, 2:
		return false
	case 3:
		select {
		case <-h.stalled:
			time.Sleep(2*tick + tick/2)
		case <-time.After(5 * time.Second):
		}
		close(h.released)
	}
	return h.halt
}

// run runs the scan of source, reads its reports up to the first processing
// one, then reads again once resume is closed or the scan returned. It
// returns the reports read once reading again, and the scan's error.
func (h *heldScan) run(t *testing.T, f scanFixture, resume <-chan struct{}, source string) ([]library.ScanProgress, error) {
	t.Helper()
	f.add(t, source)
	progress := make(chan library.ScanProgress)
	returned := make(chan error, 1)
	go func() { returned <- f.lib.Refresh([]string{source}, progress, h.stop) }()

	ticked := false
	for p := range progress {
		if p.Phase == "processing" {
			ticked = true
			break
		}
	}
	close(h.stalled)
	if !ticked {
		t.Fatal("the scan sent no processing report: stop held it outside of its processing phase")
	}

	var err error
	ended := false
	select {
	case <-resume:
	case err = <-returned:
		ended = true
	case <-time.After(10 * time.Second):
		t.Fatal("the scan waits for its reader to take a tick report")
	}
	var after []library.ScanProgress
	for p := range progress {
		after = append(after, p)
	}
	if !ended {
		err = <-returned
	}
	return after, err
}

// phases returns the phase of each report, with its count when processing.
func phases(reports []library.ScanProgress) []string {
	got := make([]string, 0, len(reports))
	for _, p := range reports {
		if p.Phase == "processing" {
			got = append(got, fmt.Sprintf("%s %d/%d", p.Phase, p.Current, p.Total))
			continue
		}
		got = append(got, p.Phase)
	}
	return got
}

// settled closes the returned channel settle after ready is closed.
func settled(ready <-chan struct{}) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		<-ready
		time.Sleep(settle)
		close(ch)
	}()
	return ch
}

func TestRefresh_AFailedWriteEndsTheScanWhileItsProgressReporterWaitsToSend(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	added := writeTrack(t, filepath.Join(source, "added.mp3"), "Added")
	f.refuse(t, insert)

	h := newHeldScan(false)
	reports, err := h.run(t, f, nil, source)

	if err == nil || !strings.Contains(err.Error(), refused) {
		t.Errorf("scan error = %v, want the refused write", err)
	}
	if len(reports) != 0 {
		t.Errorf("reports after the phase = %v, want progress closed with none", phases(reports))
	}
	if f.trackID(t, added) != 0 {
		t.Error("added.mp3 is in the library, want no track for it")
	}
}

func TestRefresh_TheLastProcessingReportOfAScanIsTheOneAtItsTotal(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	added := writeTrack(t, filepath.Join(source, "added.mp3"), "Added")

	written, quit := make(chan struct{}), make(chan struct{})
	defer close(quit)
	go func() {
		for f.trackID(t, added) == 0 {
			select {
			case <-quit:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
		close(written)
	}()
	h := newHeldScan(false)
	reports, err := h.run(t, f, settled(written), source)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	want := []string{"processing 1/1", "cleaning", "done"}
	if got := phases(reports); !slices.Equal(got, want) {
		t.Errorf("reports after the phase = %v, want %v", got, want)
	}
	if len(reports) == 0 {
		return
	}
	if last := reports[len(reports)-1]; last.Stats == nil ||
		!slices.Equal(last.Stats.BySource[source].Added, []string{"added.mp3"}) {
		t.Errorf("done report = %+v, want added.mp3 added", last)
	}
}

func TestRefresh_AScanStoppedWhileItsProgressReporterWaitsToSendReturns(t *testing.T) {
	f := newScanFixture(t)
	source := t.TempDir()
	writeTrack(t, filepath.Join(source, "skipped.mp3"), "Skipped")

	h := newHeldScan(true)
	reports, err := h.run(t, f, settled(h.released), source)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	// The phase still reports its total, though it read no file.
	want := []string{"processing 1/1"}
	if got := phases(reports); !slices.Equal(got, want) {
		t.Errorf("reports after the phase = %v, want %v", got, want)
	}
}
