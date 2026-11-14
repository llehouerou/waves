package library

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/llehouerou/waves/internal/tags"
)

// discovery is what the walk of the sources found.
type discovery struct {
	files []fileInfo
	// paths maps the path of each file found to its source.
	paths map[string]string
	// unread maps a source to the paths under it the walk could not read:
	// a root it could not list, a folder it could not list, a file it could
	// not stat, and, once markEmptyRoots ran, an empty root of a source that
	// has tracks. The tracks at and under them may still have their file.
	unread map[string][]string
	// emptyRoots holds the sources whose root has no entry at all.
	emptyRoots map[string]bool
}

// discoverFiles walks the given source directories and returns all music
// files found, with the paths it could not read.
// It ends the walk when stop says so.
func discoverFiles(sources []string, progress chan<- ScanProgress, stop func() bool) discovery {
	d := discovery{
		unread:     make(map[string][]string),
		emptyRoots: make(map[string]bool),
	}
	for _, src := range sources {
		entries, err := os.ReadDir(src)
		if err != nil {
			d.unread[src] = append(d.unread[src], src)
			continue
		}
		if len(entries) == 0 {
			d.emptyRoots[src] = true
			continue
		}
		// The callback never returns an error.
		_ = filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
			if stop() {
				return filepath.SkipAll
			}
			// A folder that could not be listed, even one that is no
			// longer there: a mount that dropped during the walk.
			if walkErr != nil {
				d.unread[src] = append(d.unread[src], path)
				return nil //nolint:nilerr // recorded as unread, the walk goes on with the other folders
			}
			if entry.IsDir() || !tags.IsMusicFile(path) {
				return nil
			}

			info, infoErr := entry.Info()
			if infoErr != nil {
				if !errors.Is(infoErr, fs.ErrNotExist) {
					d.unread[src] = append(d.unread[src], path)
				}
				return nil
			}

			d.files = append(d.files, fileInfo{
				path:   path,
				mtime:  info.ModTime().Unix(),
				source: src,
			})

			if len(d.files)%100 == 0 {
				progress <- ScanProgress{Phase: "scanning", Current: len(d.files), Total: 0}
			}
			return nil
		})
	}

	d.paths = make(map[string]string, len(d.files))
	for _, f := range d.files {
		d.paths[f.path] = f.source
	}
	return d
}

// markEmptyRoots takes the empty root of a source the library has tracks
// for, among existing (path -> mtime), for an unread root: it is what an
// unmounted mount point looks like, not a source whose files were all deleted.
func (d discovery) markEmptyRoots(existing map[string]int64) {
	for src := range d.emptyRoots {
		for path := range existing {
			if isUnder(path, src) {
				d.unread[src] = append(d.unread[src], src)
				break
			}
		}
	}
}

// isUnread reports whether path is at or under a path the walk could not read.
func (d discovery) isUnread(path string) bool {
	for _, paths := range d.unread {
		for _, unread := range paths {
			if isUnder(path, unread) {
				return true
			}
		}
	}
	return false
}

// isUnder reports whether path is dir or lies under it, on a path boundary:
// /x/music2/a.mp3 is not under /x/music.
func isUnder(path, dir string) bool {
	if path == dir {
		return true
	}
	if !strings.HasSuffix(dir, string(filepath.Separator)) {
		dir += string(filepath.Separator)
	}
	return strings.HasPrefix(path, dir)
}

// relativePath returns the path relative to the source, or the full path if not under source.
func relativePath(source, path string) string {
	rel, err := filepath.Rel(source, path)
	if err != nil {
		return path
	}
	return rel
}
