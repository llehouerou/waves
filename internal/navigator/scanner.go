package navigator

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/search"
	"github.com/llehouerou/waves/internal/tags"
)

// FileItem implements search.Item for filesystem entries.
type FileItem struct {
	Path    string // absolute path
	RelPath string // relative path for display
	IsDir   bool
}

func (f FileItem) FilterValue() string {
	return f.RelPath
}

func (f FileItem) DisplayText() string {
	if f.IsDir {
		return icons.FormatDir(f.RelPath)
	}
	return icons.FormatAudio(f.RelPath)
}

// ScanResult is sent when scanning completes or updates.
type ScanResult struct {
	Items []search.Item
	Done  bool
}

// errCanceled is returned when scanning is canceled.
var errCanceled = context.Canceled

// ScanDir scans a directory recursively and returns results via channel.
// The scan stops when the context is canceled.
func ScanDir(ctx context.Context, root string) <-chan ScanResult {
	ch := make(chan ScanResult)

	go func() {
		defer close(ch)

		var items []search.Item
		batchSize := 100

		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			// Check for cancellation
			select {
			case <-ctx.Done():
				return errCanceled
			default:
			}

			if err != nil {
				return nil //nolint:nilerr // skip permission errors, continue walking
			}

			// Skip hidden files and directories
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			// Skip root itself
			if path == root {
				return nil
			}

			// Only include directories and music files
			if d.IsDir() || tags.IsMusicFile(path) {
				relPath, _ := filepath.Rel(root, path)
				items = append(items, FileItem{
					Path:    path,
					RelPath: relPath,
					IsDir:   d.IsDir(),
				})

				// Send batch updates
				if len(items)%batchSize == 0 {
					// Check cancellation before sending
					select {
					case <-ctx.Done():
						return errCanceled
					default:
					}

					// Copy items to avoid race
					batch := make([]search.Item, len(items))
					copy(batch, items)
					ch <- ScanResult{Items: batch, Done: false}
				}
			}

			return nil
		})

		// Only send final result if not canceled
		select {
		case <-ctx.Done():
			return
		default:
			ch <- ScanResult{Items: items, Done: true}
		}
	}()

	return ch
}
