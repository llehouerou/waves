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
// The file must already exist. A FLAC, an MP3 whose ID3v2.2 tag is replaced,
// an M4A, an MP4 and an Ogg file are written whole under a temporary name
// next to the file, then renamed over it: a write that fails or is cut short
// leaves the file as it was. An M4A, MP4 or Ogg file is tagged on a copy, and
// the packets of an Ogg copy are checked against the file's before the
// rename, TagLib reporting nothing when its own writes fail. Such a file that
// is a symbolic link stays one, the file it points to is tagged. Any other
// MP3 gets a new file renamed over its path by the id3v2 library.
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
// a symbolic link, by the file build makes at the temporary path it is given,
// with the same permission bits. The new file is made under the name of the
// file plus partSuffix, created empty, then synced and renamed over the file:
// until then the file is untouched, and on an error the temporary file is
// removed. A leftover under that name is removed first.
func replaceFile(path string, build func(part string) error) error {
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
	if err := makePart(part, info.Mode().Perm(), build); err != nil {
		_ = os.Remove(part) //nolint:errcheck // the build's error is the one worth reporting
		return err
	}
	if err := os.Rename(part, target); err != nil {
		_ = os.Remove(part) //nolint:errcheck // the rename's error is the one worth reporting
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// makePart creates the file part, empty, lets build make it, then gives it the
// permission bits perm and syncs it to its device. Until then the owner may
// read and write it, whatever perm says.
func makePart(part string, perm fs.FileMode, build func(part string) error) error {
	// O_EXCL: a new file, the leftover having been removed. Nothing here
	// serialises two writes of the same track: they must not run at once.
	f, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm|0o600)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	if err := build(part); err != nil {
		return err
	}
	// Synced after build: the libraries that tag the file write it through
	// handles of their own.
	f, err = os.OpenFile(part, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open temporary file: %w", err)
	}
	// The umask may have taken bits off perm.
	if err := f.Chmod(perm); err != nil {
		_ = f.Close() //nolint:errcheck // the chmod's error is the one worth reporting
		return fmt.Errorf("set permissions: %w", err)
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

// buildByWriting returns a build of replaceFile that writes the new file with
// write.
func buildByWriting(write func(io.Writer) error) func(part string) error {
	return func(part string) error {
		f, err := os.OpenFile(part, os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("open temporary file: %w", err)
		}
		if err := write(f); err != nil {
			_ = f.Close() //nolint:errcheck // the write's error is the one worth reporting
			return fmt.Errorf("write: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close: %w", err)
		}
		return nil
	}
}

// replaceWithTaggedCopy replaces the file at path (replaceFile) by a copy of
// it that tag tags at the temporary path it is given.
func replaceWithTaggedCopy(path string, tag func(part string) error) error {
	return replaceFile(path, func(part string) error {
		if err := buildByWriting(func(w io.Writer) error {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(w, f)
			return err
		})(part); err != nil {
			return fmt.Errorf("copy: %w", err)
		}
		return tag(part)
	})
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
