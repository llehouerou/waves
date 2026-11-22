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
	"strings"
	"syscall"
	"testing"

	"go.senan.xyz/taglib"
)

// The environment of a test binary run to write tags under a file size limit.
const (
	limitedWritePath  = "WAVES_TAGS_LIMITED_WRITE_PATH"
	limitedWriteLimit = "WAVES_TAGS_LIMITED_WRITE_LIMIT"
	limitedWriteCover = "WAVES_TAGS_LIMITED_WRITE_COVER"
)

// limitedWriteFailed is the exit code of a limited write whose Write returned
// an error.
const limitedWriteFailed = 3

func TestMain(m *testing.M) {
	if path := os.Getenv(limitedWritePath); path != "" {
		os.Exit(limitedWrite(path, os.Getenv(limitedWriteLimit), os.Getenv(limitedWriteCover)))
	}
	os.Exit(m.Run())
}

// limitedWrite writes newTags, with the cover in the file cover if one is
// named, on the track at path with files limited to limit bytes, as a full
// disk refuses a write partway, and returns the exit code.
func limitedWrite(path, limit, cover string) int {
	tag := newTags()
	if cover != "" {
		data, err := os.ReadFile(cover)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cover:", err)
			return 1
		}
		tag.CoverArt = data
	}
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
	if err := Write(path, tag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return limitedWriteFailed
	}
	return 0
}

// writeUnderLimit runs Write on the track at path in a child process whose
// files cannot grow past limit bytes, and returns what Write reported.
func writeUnderLimit(t *testing.T, path string, limit int) error {
	t.Helper()
	return writeUnderLimitWithCover(t, path, limit, nil)
}

// writeUnderLimitWithCover is writeUnderLimit with cover written too, when
// it is not nil.
func writeUnderLimitWithCover(t *testing.T, path string, limit int, cover []byte) error {
	t.Helper()
	cmd := exec.Command(os.Args[0]) //nolint:gosec // the test binary itself
	cmd.Env = append(os.Environ(),
		limitedWritePath+"="+path,
		limitedWriteLimit+"="+strconv.Itoa(limit),
	)
	if cover != nil {
		coverPath := filepath.Join(t.TempDir(), "cover.jpg")
		if err := os.WriteFile(coverPath, cover, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd.Env = append(cmd.Env, limitedWriteCover+"="+coverPath)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ExtM4A, ExtMP4:
		// go-mp4tag writes the whole new file in the system's temporary
		// folder first, and leaves it there when the write is refused.
		cmd.Env = append(cmd.Env, "TMPDIR="+t.TempDir())
	case ExtOPUS, ExtOGG, ExtOGA:
		// TagLib compiles itself into the system's temporary folder the
		// first time: under the limit, with no cache yet, it would fail
		// before it writes anything.
		if _, err := taglib.ReadTags(path); err != nil && !errors.Is(err, taglib.ErrInvalidFile) {
			t.Fatalf("warm TagLib up: %v", err)
		}
	}
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

func TestWrite_OggCopyTagLibFailedToWriteIsRefused(t *testing.T) {
	for _, fx := range oggKindsWithCover {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			path := fx.make(t, dir)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			// Room for the copy of the track, not for the cover: TagLib
			// writes the copy cut short and reports nothing.
			err = writeUnderLimitWithCover(t, path, len(before)+50000, testCover(240*1024))
			if err == nil {
				t.Fatal("Write() of a copy TagLib could not write succeeded, want an error")
			}
			if !strings.Contains(err.Error(), "check tagged copy") {
				t.Errorf("Write() error = %q, want one from the check of the tagged copy", err)
			}

			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, before) {
				t.Errorf("the refused write changed the file: %d bytes, was %d", len(after), len(before))
			}
			assertOnlyFile(t, dir, filepath.Base(path))
		})
	}
}

func TestWrite_OggCoverNotWrittenLeavesOldTagsAndCover(t *testing.T) {
	for _, fx := range oggKindsWithCover {
		t.Run(fx.name, func(t *testing.T) {
			dir := t.TempDir()
			path := fx.make(t, dir)
			oldCover := testCover(10 * 1024)
			if err := Write(path, &Tag{Title: "Old Title", Artist: "Old Artist", CoverArt: oldCover}); err != nil {
				t.Fatalf("Write() of the old tags: %v", err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			// Room for the new tags, not for the new cover.
			if err := writeUnderLimitWithCover(t, path, len(before)+50000, testCover(240*1024)); err == nil {
				t.Fatal("Write() of a cover that could not be written succeeded, want an error")
			}

			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(after, before) {
				t.Errorf("the failed write changed the file: %d bytes, was %d", len(after), len(before))
			}
			got, err := Read(path)
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}
			if got.Title != "Old Title" {
				t.Errorf("Title = %q, want the old one", got.Title)
			}
			cover, err := taglib.ReadImage(path)
			if err != nil {
				t.Fatalf("ReadImage() error: %v", err)
			}
			if !bytes.Equal(cover, oldCover) {
				t.Errorf("the cover is %d bytes, want the old one of %d", len(cover), len(oldCover))
			}
			assertOnlyFile(t, dir, filepath.Base(path))
		})
	}
}
