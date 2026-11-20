// Package linkfs reads folders the way waves sees them: an entry that is a
// symbolic link is what it points to, and is named by its own path, never by
// the path of its target; an entry whose name starts with a dot is hidden:
// the Files view, its deep search, queueing a folder and a library scan leave
// it out. Nothing here resolves a path.
package linkfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// errDangling is the error of Info for a symbolic link whose target is
// missing. It is not fs.ErrNotExist: the link is there.
var errDangling = errors.New("symbolic link to nothing")

// Kind is what an entry is, a symbolic link taken for what it points to.
type Kind int

const (
	// None is a symbolic link whose target is missing or cannot be read.
	None Kind = iota
	// Dir is a directory.
	Dir
	// File is anything else.
	File
)

// isLink reports whether entry is a symbolic link.
func isLink(entry fs.DirEntry) bool {
	return entry.Type()&fs.ModeSymlink != 0
}

// KindOf returns what the entry at path, from a listing or a walk of its
// folder, is. Only an entry that is a symbolic link costs a stat.
func KindOf(path string, entry fs.DirEntry) Kind {
	if !isLink(entry) {
		if entry.IsDir() {
			return Dir
		}
		return File
	}
	info, err := os.Stat(path)
	switch {
	case err != nil:
		return None
	case info.IsDir():
		return Dir
	default:
		return File
	}
}

// Info returns the file info of the entry at path, from a listing or a walk
// of its folder: the entry's own, or what a symbolic link points to. A link
// whose target is missing gives an error that is not fs.ErrNotExist; an entry no longer there gives
// fs.ErrNotExist.
func Info(path string, entry fs.DirEntry) (fs.FileInfo, error) {
	if !isLink(entry) {
		return entry.Info()
	}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		if _, lerr := os.Lstat(path); lerr == nil {
			return nil, fmt.Errorf("%s: %w", path, errDangling)
		}
	}
	return info, err
}

// Hidden reports whether an entry named name is hidden: its name starts with
// a dot. waves lists, searches and queues no hidden entry below the folder it
// is asked for, whatever the name of that folder, and a library scan takes no
// hidden entry below the root of a source.
func Hidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

// WalkShown walks the tree at root as Walk does, leaving out the hidden
// entries below root: fn gets no hidden file, and no hidden folder nor
// anything under it. root is walked whatever its name.
func WalkShown(root string, fn fs.WalkDirFunc) error {
	return Walk(root, func(path string, entry fs.DirEntry, err error) error {
		if path != root && entry != nil && Hidden(entry.Name()) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		return fn(path, entry, err)
	})
}

// WithSeparator returns dir ending with one path separator: what the path of
// everything under dir starts with, and the spelling of dir that names the
// directory it points to when dir is a symbolic link.
func WithSeparator(dir string) string {
	sep := string(filepath.Separator)
	return strings.TrimSuffix(dir, sep) + sep
}

// Walk walks the tree at root as filepath.WalkDir does, but walks a root that
// is a symbolic link to a directory as that directory. fn gets root as given
// for the root, and the paths under it joined to it, cleaned: under the link,
// never under the directory it points to. Below the root, no symbolic link is
// followed: fn gets its entry, which KindOf and Info see through.
func Walk(root string, fn fs.WalkDirFunc) error {
	// WalkDir follows no symbolic link, its root included, but the path of a
	// link followed by a separator names the directory it points to.
	// An empty root names no directory, and not the root of the system.
	if root == "" {
		return filepath.WalkDir(root, fn)
	}
	walked := WithSeparator(root)
	return filepath.WalkDir(walked, func(path string, entry fs.DirEntry, err error) error {
		if path == walked {
			path = root
		}
		return fn(path, entry, err)
	})
}
