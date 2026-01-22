package library

import (
	"os"
	"path/filepath"

	"github.com/llehouerou/waves/internal/tags"
)

// discoverFiles walks the given source directories and returns all music files found.
// Returns the list of files and a map of path->source for quick lookup.
func discoverFiles(sources []string, progress chan<- ScanProgress) (files []fileInfo, discoveredPaths map[string]string) {
	for _, src := range sources {
		_ = filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
			// Skip any walk errors - intentionally continuing to scan other paths
			if walkErr != nil {
				return nil //nolint:nilerr // intentionally skipping errors
			}
			if d.IsDir() {
				return nil
			}
			if !tags.IsMusicFile(path) {
				return nil
			}

			info, infoErr := d.Info()
			// Skip files we can't stat - intentionally continuing to scan other files
			if infoErr != nil {
				return nil //nolint:nilerr // intentionally skipping errors
			}

			files = append(files, fileInfo{
				path:   path,
				mtime:  info.ModTime().Unix(),
				source: src,
			})

			if len(files)%100 == 0 {
				progress <- ScanProgress{Phase: "scanning", Current: len(files), Total: 0}
			}
			return nil
		})
	}

	// Build set of discovered paths for deletion phase (with source info)
	discoveredPaths = make(map[string]string, len(files)) // path -> source
	for _, f := range files {
		discoveredPaths[f.path] = f.source
	}

	return files, discoveredPaths
}

// relativePath returns the path relative to the source, or the full path if not under source.
func relativePath(source, path string) string {
	rel, err := filepath.Rel(source, path)
	if err != nil {
		return path
	}
	return rel
}
