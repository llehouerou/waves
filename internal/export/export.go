package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Exporter handles copying and converting files for export.
type Exporter struct{}

// NewExporter creates a new Exporter.
func NewExporter() *Exporter {
	return &Exporter{}
}

// NeedsConversion returns true if the file extension requires conversion.
func NeedsConversion(ext string) bool {
	return strings.EqualFold(ext, ".flac")
}

// partSuffix ends the name a track is written under until it is whole: not an
// audio extension, so that no player takes it for a track.
const partSuffix = ".part"

// CopyFile copies a file from src to dst.
// Creates parent directories if needed.
// Skips if destination already exists.
// dst exists only once whole: the copy is written next to it, synced, then
// renamed to it.
func (e *Exporter) CopyFile(src, dst string) error {
	return e.copyFile(context.Background(), src, dst)
}

// copyFile is CopyFile, whose wait for another export writing dst ends with
// ctx. Once begun, the copy is finished whatever ctx says.
func (e *Exporter) copyFile(ctx context.Context, src, dst string) error {
	return writeWhole(ctx, dst, func(part string) error {
		srcFile, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("open source: %w", err)
		}
		defer srcFile.Close()

		// O_EXCL: a new file, never the leftover a killed waves may still write.
		dstFile, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return fmt.Errorf("create destination: %w", err)
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return fmt.Errorf("copy: %w", err)
		}
		return syncAndClose(dstFile)
	})
}

// ConvertToMP3 converts a FLAC file to MP3 using ffmpeg.
// Uses 320kbps CBR preset.
// dst exists only once whole, as for CopyFile.
// The end of ctx kills ffmpeg, and what it wrote is removed.
func (e *Exporter) ConvertToMP3(ctx context.Context, src, dst string) error {
	return writeWhole(ctx, dst, func(part string) error {
		cmd := exec.CommandContext(ctx, "ffmpeg",
			"-i", src,
			"-codec:a", "libmp3lame",
			"-b:a", "320k",
			"-map_metadata", "0", // Preserve tags
			"-id3v2_version", "3",
			"-f", "mp3", // the name it writes does not end in .mp3
			"-n", // never write into a file already there
			part,
		)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ffmpeg conversion failed: %w\n%s", err, string(output))
		}
		return syncFile(part)
	})
}

// writeWhole writes dst through fill, which writes a new file at the name it is
// given, next to dst: dst gets that file once fill returns, and nothing when
// fill fails. A dst already there is left as it is.
//
// The leftover of a write cut short is removed first, not written into: the
// process that left it may still write it. Two exports of this process that
// reach dst write it one after the other; the end of ctx ends the wait.
func writeWhole(ctx context.Context, dst string, fill func(part string) error) error {
	release, err := writing.hold(ctx, dst)
	if err != nil {
		return err
	}
	defer release()

	if _, err := os.Stat(dst); err == nil {
		return nil // Already exists, skip
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	part := dst + partSuffix
	if err := os.Remove(part); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("temporary file: %w", err)
	}
	if err := fill(part); err != nil {
		_ = os.Remove(part) //nolint:errcheck // the write's error is the one worth reporting
		return err
	}
	if err := os.Rename(part, dst); err != nil {
		_ = os.Remove(part) //nolint:errcheck // the rename's error is the one worth reporting
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// syncFile makes the content of the file at path reach its device.
func syncFile(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	return syncAndClose(f)
}

// syncAndClose makes the content of f reach its device, then closes it.
func syncAndClose(f *os.File) error {
	if err := f.Sync(); err != nil {
		_ = f.Close() //nolint:errcheck // the sync's error is the one worth reporting
		return fmt.Errorf("sync: %w", err)
	}
	return f.Close()
}

// writing is the destinations the exports of this process write.
var writing = destinations{held: make(map[string]chan struct{})}

// destinations hands each destination to one writer at a time.
type destinations struct {
	mu   sync.Mutex
	held map[string]chan struct{} // closed when the destination is released
}

// hold waits until no one else writes dst, and holds it until release is
// called. The end of ctx ends the wait, with its error.
func (d *destinations) hold(ctx context.Context, dst string) (release func(), err error) {
	for {
		d.mu.Lock()
		busy, ok := d.held[dst]
		if !ok {
			released := make(chan struct{})
			d.held[dst] = released
			d.mu.Unlock()
			return func() {
				d.mu.Lock()
				delete(d.held, dst)
				d.mu.Unlock()
				close(released)
			}, nil
		}
		d.mu.Unlock()
		select {
		case <-busy:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// ExportFile exports a single track, converting if needed.
func (e *Exporter) ExportFile(ctx context.Context, src, dst string, convert bool) error {
	ext := strings.ToLower(filepath.Ext(src))

	if convert && NeedsConversion(ext) {
		// Change extension to .mp3
		dst = strings.TrimSuffix(dst, filepath.Ext(dst)) + ".mp3"
		return e.ConvertToMP3(ctx, src, dst)
	}

	return e.copyFile(ctx, src, dst)
}
