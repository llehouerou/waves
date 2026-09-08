//go:build unix

package albumart

import (
	"os"
	"testing"
	"time"
)

// os.File deadlines do not work on stdin (Go never switches it to non-blocking
// mode), so the probe polls the descriptor itself. Getting this wrong silently
// turns every terminal into "no graphics support" (issue #30).
func TestWaitReadable_WaitsForData(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	if waitReadable(r.Fd(), 20*time.Millisecond) {
		t.Fatal("waitReadable reported data on an empty pipe")
	}

	if _, err := w.WriteString("x"); err != nil {
		t.Fatal(err)
	}
	if !waitReadable(r.Fd(), time.Second) {
		t.Error("waitReadable missed data ready to be read")
	}
}

func TestWaitReadable_HonoursTimeout(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	start := time.Now()
	waitReadable(r.Fd(), 50*time.Millisecond)

	if elapsed := time.Since(start); elapsed < 40*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Errorf("waited %v for a 50ms timeout", elapsed)
	}
}

// The reader feeds readProbeReply, so it must report EOF-like behaviour rather
// than block once the deadline has passed.
func TestDeadlineReader_StopsAtDeadline(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	if _, err := w.WriteString("\x1b[?6c"); err != nil {
		t.Fatal(err)
	}

	got := readProbeReply(newDeadlineReader(r.Fd(), 100*time.Millisecond))

	if got != "\x1b[?6c" {
		t.Errorf("readProbeReply = %q, want the full reply", got)
	}
}

func TestDeadlineReader_ReturnsWithoutData(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	start := time.Now()
	got := readProbeReply(newDeadlineReader(r.Fd(), 50*time.Millisecond))

	if got != "" {
		t.Errorf("readProbeReply = %q, want empty", got)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("silent terminal cost %v, want ~50ms", elapsed)
	}
}
