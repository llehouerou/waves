package player

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// countingFile counts the reads that reach the real file.
type countingFile struct {
	*os.File
	reads int
	bytes int64
}

func (c *countingFile) ReadAt(p []byte, off int64) (int, error) {
	c.reads++
	n, err := c.File.ReadAt(p, off)
	c.bytes += int64(n)
	return n, err
}

func openCounted(t *testing.T, path string) (*bufferedFile, *countingFile) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c := &countingFile{File: f}
	b, err := newBufferedFile(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return b, c
}

func TestBufferedFileCollapsesReads(t *testing.T) {
	// 1 MB read in 265-byte chunks (the MP3 decoder's pattern): unbuffered
	// that is ~4000 syscalls.
	path := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(path, make([]byte, 1<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	b, c := openCounted(t, path)

	chunk := make([]byte, 265)
	for {
		if _, err := b.Read(chunk); err != nil {
			break
		}
	}
	if c.reads > 5 {
		t.Errorf("reading 1 MB in 265-byte chunks hit the file %d times, want <= 5", c.reads)
	}
}

func TestBufferedFileShortBackwardSeekIsFree(t *testing.T) {
	// go-mp3 reads a frame then seeks back a few bytes, thousands of times.
	// That must not re-read the file.
	path := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(path, make([]byte, 1<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	b, c := openCounted(t, path)

	chunk := make([]byte, 265)
	for range 2000 {
		if _, err := b.Read(chunk); err != nil {
			break
		}
		if _, err := b.Seek(-4, io.SeekCurrent); err != nil {
			t.Fatal(err)
		}
	}
	if c.bytes > 2<<20 {
		t.Errorf("read %d bytes off a 1 MB file, want no re-reading", c.bytes)
	}
}

func TestBufferedFileSeek(t *testing.T) {
	src := "testdata/vorbis_44100_stereo.ogg"
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := openCounted(t, src)

	got := make([]byte, 16)
	for _, off := range []int64{0, 100, 4096, 50, int64(len(want)) - 16} {
		if _, err := b.Seek(off, io.SeekStart); err != nil {
			t.Fatalf("seek %d: %v", off, err)
		}
		if _, err := io.ReadFull(b, got); err != nil {
			t.Fatalf("read at %d: %v", off, err)
		}
		if !bytes.Equal(got, want[off:off+16]) {
			t.Errorf("wrong bytes at offset %d", off)
		}
	}

	end, err := b.Seek(0, io.SeekEnd)
	if err != nil || end != int64(len(want)) {
		t.Fatalf("SeekEnd = %d, %v; want %d", end, err, len(want))
	}
	if _, err := b.Read(got); !errors.Is(err, io.EOF) {
		t.Errorf("read at EOF = %v, want io.EOF", err)
	}
	if _, err := b.Seek(-16, io.SeekCurrent); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(b, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want[len(want)-16:]) {
		t.Error("wrong bytes after SeekCurrent")
	}
}

func TestBufferedFileDecodesOgg(t *testing.T) {
	b, c := openCounted(t, "testdata/vorbis_44100_stereo.ogg")
	src, _, err := decodeOgg(b)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	buf := make([][2]float64, 512)
	for {
		if n, ok := src.Stream(buf); !ok || n == 0 {
			break
		}
	}
	if c.reads > 2 {
		t.Errorf("decoding a 7.6 KB file hit the file %d times, want <= 2", c.reads)
	}
}
