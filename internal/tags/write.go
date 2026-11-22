package tags

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Write writes tag metadata to a music file.
// The file must already exist. A FLAC, and an MP3 whose ID3v2.2 tag is
// replaced, are written whole under a temporary name next to the file, then
// renamed over it: a write that fails or is cut short leaves the file as it
// was. Such a file that is a symbolic link stays one, the file it points to
// is tagged. Any other MP3 gets a new file renamed over its path by the id3v2
// library; an M4A, MP4 or Ogg file is modified in place.
func Write(path string, t *Tag) error {
	// Check file exists
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Detect file format from extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ExtMP3 && ext != ExtFLAC && ext != ExtOPUS && ext != ExtOGG && ext != ExtOGA && ext != ExtM4A && ext != ExtMP4 {
		return fmt.Errorf("unsupported file format: %s", ext)
	}

	switch ext {
	case ExtMP3:
		return writeMP3Tags(path, t)
	case ExtFLAC:
		return writeFLACTags(path, t)
	case ExtOPUS, ExtOGG, ExtOGA:
		return writeOggTags(path, t)
	case ExtM4A, ExtMP4:
		return writeM4ATags(path, t)
	}

	return nil
}

// partSuffix ends the name a new file of a track is written under until it
// replaces the track: not a music extension, so that IsMusicFile refuses the
// leftover of a write cut short.
const partSuffix = ".part"

// replaceFile replaces the file at path, or the file it points to when it is
// a symbolic link, by what write writes, with the same permission bits. The
// new file is written under the name of the file plus partSuffix, synced, then
// renamed over the file: until then the file is untouched, and on an error
// the temporary file is removed. A leftover under that name is removed first.
func replaceFile(path string, write func(io.Writer) error) error {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve file: %w", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	part := target + partSuffix
	if err := os.Remove(part); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove leftover: %w", err)
	}
	if err := writePart(part, info.Mode().Perm(), write); err != nil {
		_ = os.Remove(part) //nolint:errcheck // the write's error is the one worth reporting
		return err
	}
	if err := os.Rename(part, target); err != nil {
		_ = os.Remove(part) //nolint:errcheck // the rename's error is the one worth reporting
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// writePart creates the file part with the permission bits perm, writes it
// with write and syncs it to its device.
func writePart(part string, perm fs.FileMode, write func(io.Writer) error) error {
	// O_EXCL: a new file, the leftover having been removed. Nothing here
	// serialises two writes of the same track: they must not run at once.
	f, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	// The umask may have taken bits off perm.
	if err := f.Chmod(perm); err != nil {
		_ = f.Close() //nolint:errcheck // the chmod's error is the one worth reporting
		return fmt.Errorf("set permissions: %w", err)
	}
	if err := write(f); err != nil {
		_ = f.Close() //nolint:errcheck // the write's error is the one worth reporting
		return fmt.Errorf("write: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close() //nolint:errcheck // the sync's error is the one worth reporting
		return fmt.Errorf("sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return nil
}

const (
	mimeJPEG = "image/jpeg"
	mimePNG  = "image/png"
)

// detectMimeType detects the MIME type of image data.
func detectMimeType(data []byte) string {
	if len(data) == 0 {
		return mimeJPEG
	}
	contentType := http.DetectContentType(data)
	// http.DetectContentType may return more specific types, normalize to common ones
	switch contentType {
	case mimeJPEG:
		return mimeJPEG
	case mimePNG:
		return mimePNG
	default:
		// Default to JPEG for unknown types
		return mimeJPEG
	}
}
