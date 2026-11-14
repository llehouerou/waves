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
	// not stat, and, once markSourcesWithoutMusic ran, the root of a source
	// in which it found no music file while the library has tracks for it.
	// The tracks at and under them may still have their file.
	unread map[string][]string
	// sources are the sources walked.
	sources []string
}

// discoverFiles walks the given source directories and returns all music
// files found, with the paths it could not read.
// It ends the walk when stop says so.
func discoverFiles(sources []string, progress chan<- ScanProgress, stop func() bool) discovery {
	d := discovery{
		unread:  make(map[string][]string),
		sources: sources,
	}
	for _, src := range sources {
		// The walk would report a missing or unlistable root too, but not a
		// root that is not a directory: it would visit it as a file.
		if _, err := os.ReadDir(src); err != nil {
			d.unread[src] = append(d.unread[src], src)
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

// markSourcesWithoutMusic takes the root of a source in which the walk found
// no music file for an unread root, when the library has tracks for it among
// existing (path -> mtime) that the scan would otherwise remove: it is what
// an unmounted mount point looks like, empty or not, not a source whose files
// were all deleted.
func (d discovery) markSourcesWithoutMusic(existing map[string]int64) {
	withMusic := make(map[string]bool)
	for _, f := range d.files {
		withMusic[f.source] = true
	}
	for _, src := range d.sources {
		if withMusic[src] {
			continue
		}
		for path := range existing {
			if isUnder(path, src) && d.isGone(path) {
				d.unread[src] = append(d.unread[src], src)
				break
			}
		}
	}
}

// isGone reports whether the file of the track at path is known to be gone:
// the walk did not find it and could read every path it may be at.
func (d discovery) isGone(path string) bool {
	_, found := d.paths[path]
	return !found && !d.isUnread(path)
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
	return strings.HasPrefix(path, withSeparator(dir))
}

// withSeparator returns dir ending with one path separator: what the path of
// everything under dir starts with. sourceBounds selects in SQL what it does.
func withSeparator(dir string) string {
	sep := string(filepath.Separator)
	return strings.TrimSuffix(dir, sep) + sep
}

// sourceOf returns the first of sources that path is at or under.
func sourceOf(path string, sources []string) (string, bool) {
	for _, src := range sources {
		if isUnder(path, src) {
			return src, true
		}
	}
	return "", false
}

// relativePath returns the path relative to the source, or the full path if not under source.
func relativePath(source, path string) string {
	rel, err := filepath.Rel(source, path)
	if err != nil {
		return path
	}
	return rel
}
