package library

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/llehouerou/waves/internal/linkfs"
	"github.com/llehouerou/waves/internal/tags"
)

// discovery is what the walk of the sources found.
// Each file and each unread path is found once, under its innermost source.
type discovery struct {
	files []fileInfo
	// paths maps the path of each file found to its source.
	paths map[string]string
	// unread maps a source to the paths the walk could not read whose
	// innermost source it is:
	// a root it could not list, a folder it could not list, a file it could
	// not stat, and, once markSourcesWithoutMusic ran, the root of a source
	// in which it found no music file while the library has tracks for it.
	// The tracks at and under them may still have their file.
	unread map[string][]string
	// sources are the sources walked.
	sources []string
	// librarySources are the sources of the library when the walk began,
	// among which the hidden rule takes the root of a path.
	librarySources []string
}

// discoverFiles walks the given source directories and returns all music
// files found, with the paths it could not read, each once, under the
// innermost of sources that holds it: the walk of a source leaves the folders
// of the sources inside it to their own walk.
// A source that is a symbolic link to a directory is walked as that
// directory, and the paths found stay under the source as given
// (<link>/a/one.mp3), never under the directory it points to. Only the root
// is followed: a symbolic link to a folder inside a source is not. A music
// file that is a link is found with the modification time of its target.
//
// It leaves out the entries leftOut says: no hidden file is found, unread or
// counted in progress, and no hidden folder is walked, unless it holds the
// root of a source of the library, which it walks to reach that root.
// It ends the walk when stop says so.
func discoverFiles(sources, librarySources []string, progress chan<- ScanProgress, stop func() bool) discovery {
	d := discovery{
		unread:         make(map[string][]string),
		sources:        sources,
		librarySources: librarySources,
	}
	for _, src := range sources {
		// The walk would report a missing or unlistable root too, but not a
		// root that is not a directory: it would visit it as a file.
		if _, err := os.ReadDir(src); err != nil {
			d.unread[src] = append(d.unread[src], src)
			continue
		}
		// The callback never returns an error.
		_ = linkfs.Walk(src, func(path string, entry os.DirEntry, walkErr error) error {
			if stop() {
				return filepath.SkipAll
			}
			// A hidden entry, a symbolic link among them: nothing the scan
			// would read, even in a folder it could not list.
			if d.leftOut(path) {
				if walkErr == nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// A folder that could not be listed, even one that is no
			// longer there: a mount that dropped during the walk.
			if walkErr != nil {
				d.unread[src] = append(d.unread[src], path)
				return nil //nolint:nilerr // recorded as unread, the walk goes on with the other folders
			}
			if entry.IsDir() {
				return walkedBy(path, src, sources)
			}

			info, isMusic, infoErr := musicFile(path, entry)
			if infoErr != nil {
				if !errors.Is(infoErr, fs.ErrNotExist) {
					d.unread[src] = append(d.unread[src], path)
				}
				return nil
			}
			if !isMusic {
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

// walkedBy returns what the walk of src does with the folder at path: a
// folder under a source inside src, spelled with or without its trailing
// separator, is walked by that source and skipped here.
func walkedBy(path, src string, sources []string) error {
	if inner, _ := sourceOf(linkfs.WithSeparator(path), sources); inner != src {
		return filepath.SkipDir
	}
	return nil
}

// musicFile returns the file info of the entry at path, from a walk, and
// whether it is a music file. A symbolic link is what it points to: a link
// to nothing (a disk not mounted) gives an error that is not
// fs.ErrNotExist, and a link to a folder is no music file.
func musicFile(path string, entry os.DirEntry) (fs.FileInfo, bool, error) {
	if !tags.IsMusicFile(path) {
		return nil, false, nil
	}
	info, err := linkfs.Info(path, entry)
	if err != nil {
		return nil, false, err
	}
	return info, !info.IsDir(), nil
}

// markSourcesWithoutMusic takes the root of a source in which the walk found
// no music file for an unread root, when the library has tracks for it among
// existing (path -> mtime) that the scan would otherwise remove: it is what
// an unmounted mount point looks like, empty or not, not a source whose files
// were all deleted. A music file anywhere under a source counts for it, in a
// source inside it or not, hidden or not: the hidden entries the walk left
// out are looked through for one only then, until stop says so. Each source
// is decided on what the walk read, before any root is marked.
func (d discovery) markSourcesWithoutMusic(existing map[string]int64, stop func() bool) {
	var marked []string
	for _, src := range d.sources {
		if d.hasMusic(src) {
			continue
		}
		withGone := false
		for path := range existing {
			if isUnder(path, src) && d.isGone(path) {
				withGone = true
				break
			}
		}
		if withGone && !d.hasHiddenMusic(src, stop) {
			marked = append(marked, src)
		}
	}
	for _, src := range marked {
		d.unread[src] = append(d.unread[src], src)
	}
}

// hasMusic reports whether the walk found a music file under src.
func (d discovery) hasMusic(src string) bool {
	for _, f := range d.files {
		if isUnder(f.path, src) {
			return true
		}
	}
	return false
}

// hasHiddenMusic reports whether a music file lies under src, hidden or
// not, walking src and the sources inside it as discoverFiles does, each the
// root of its own walk, but without leaving out what is hidden. What it
// cannot read holds none. It ends the walk when stop says so.
func (d discovery) hasHiddenMusic(src string, stop func() bool) bool {
	found := false
	for _, root := range d.sources {
		if found || !isUnder(root, src) {
			continue
		}
		// The callback never returns an error.
		_ = linkfs.Walk(root, func(path string, entry os.DirEntry, walkErr error) error {
			if stop() {
				return filepath.SkipAll
			}
			if walkErr != nil {
				return nil //nolint:nilerr // what cannot be read holds no music file
			}
			if entry.IsDir() {
				return walkedBy(path, root, d.sources)
			}
			if _, isMusic, err := musicFile(path, entry); err == nil && isMusic {
				found = true
				return filepath.SkipAll
			}
			return nil
		})
	}
	return found
}

// isHidden reports whether path is hidden for the scan, as hiddenUnder says
// among the sources of the library when the walk began.
func (d discovery) isHidden(path string) bool {
	return hiddenUnder(path, d.librarySources)
}

// leftOut reports whether the walk leaves out the entry at path: it is
// hidden and holds the root of no source of the library.
func (d discovery) leftOut(path string) bool {
	if !d.isHidden(path) {
		return false
	}
	return !slices.ContainsFunc(d.librarySources, func(src string) bool {
		return src != path && isUnder(src, path)
	})
}

// hiddenUnder reports whether path is hidden for the scan: one of its names
// below the root of the innermost of sources that holds it is hidden, as
// linkfs.Hidden says. The root is taken whatever its name and the names of
// the folders above it; a path no source holds is not hidden.
func hiddenUnder(path string, sources []string) bool {
	src, ok := sourceOf(path, sources)
	if !ok {
		return false
	}
	rel, ok := strings.CutPrefix(path, linkfs.WithSeparator(src))
	if !ok {
		return false
	}
	return slices.ContainsFunc(strings.Split(rel, string(filepath.Separator)), linkfs.Hidden)
}

// isGone reports whether the file of the track at path is known to be gone:
// the walk did not find it and could read every path it may be at. A track
// at a hidden path is taken for gone, its file there or not: the walk leaves
// it out.
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
	return strings.HasPrefix(path, linkfs.WithSeparator(dir))
}

// sourceOf returns the innermost of sources that path is at or under: the
// one with the longest path.
func sourceOf(path string, sources []string) (string, bool) {
	found := false
	var innermost string
	for _, src := range sources {
		if isUnder(path, src) && (!found || len(src) > len(innermost)) {
			innermost, found = src, true
		}
	}
	return innermost, found
}

// relativePath returns the path relative to the source, or the full path if not under source.
func relativePath(source, path string) string {
	rel, err := filepath.Rel(source, path)
	if err != nil {
		return path
	}
	return rel
}
