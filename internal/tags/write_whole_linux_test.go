//go:build linux

package tags

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
)

// The environment of a test binary run to write tags under a file size limit.
const (
	limitedWritePath  = "WAVES_TAGS_LIMITED_WRITE_PATH"
	limitedWriteLimit = "WAVES_TAGS_LIMITED_WRITE_LIMIT"
)

// limitedWriteFailed is the exit code of a limited write whose Write returned
// an error.
const limitedWriteFailed = 3

func TestMain(m *testing.M) {
	if path := os.Getenv(limitedWritePath); path != "" {
		os.Exit(limitedWrite(path, os.Getenv(limitedWriteLimit)))
	}
	os.Exit(m.Run())
}

// limitedWrite writes newTags on the track at path with files limited to limit
// bytes, as a full disk refuses a write partway, and returns the exit code.
func limitedWrite(path, limit string) int {
	n, err := strconv.ParseUint(limit, 10, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "limit:", err)
		return 1
	}
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &rlimit); err != nil {
		fmt.Fprintln(os.Stderr, "get limit:", err)
		return 1
	}
	rlimit.Cur = n
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &rlimit); err != nil {
		fmt.Fprintln(os.Stderr, "set limit:", err)
		return 1
	}
	// Past the limit, a write fails instead of killing the process.
	signal.Ignore(syscall.SIGXFSZ)
	if err := Write(path, newTags()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return limitedWriteFailed
	}
	return 0
}

// writeUnderLimit runs Write on the track at path in a child process whose
// files cannot grow past limit bytes, and returns what Write reported.
func writeUnderLimit(t *testing.T, path string, limit int) error {
	t.Helper()
	cmd := exec.Command(os.Args[0]) //nolint:gosec // the test binary itself
	cmd.Env = append(os.Environ(),
		limitedWritePath+"="+path,
		limitedWriteLimit+"="+strconv.Itoa(limit),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != limitedWriteFailed {
		t.Fatalf("limited write: %v: %s", err, stderr.String())
	}
	return errors.New(stderr.String())
}

func TestWrite_RefusedPartwayLeavesTrackAsItWas(t *testing.T) {
	for _, fx := range wholeWriteFixtures {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			path := fx.make(t, dir)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			audio := trackAudio(t, path)

			if err := writeUnderLimit(t, path, len(before)/2); err == nil {
				t.Fatal("Write() refused partway succeeded, want an error")
			}

			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, before) {
				t.Errorf("the refused write changed the file: %d bytes, was %d", len(after), len(before))
			}
			assertOnlyFile(t, dir, filepath.Base(path))

			// The next write starts from the intact track.
			if err := Write(path, newTags()); err != nil {
				t.Fatalf("Write() after the refused one: %v", err)
			}
			assertNewTags(t, path, audio)
			assertOnlyFile(t, dir, filepath.Base(path))
		})
	}
}
