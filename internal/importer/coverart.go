package importer

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/llehouerou/waves/internal/tags"
)

// WriteCoverArt writes cover art as cover.jpg or cover.png in an album folder,
// unless the folder already has cover art of its own (cover, folder, front…,
// any case), which is never replaced. Nothing to write is not an error.
func WriteCoverArt(albumDir string, data []byte) error {
	if len(data) == 0 || tags.HasFolderArt(albumDir) {
		return nil
	}
	ext := ".jpg"
	if http.DetectContentType(data) == "image/png" {
		ext = ".png"
	}
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(albumDir, "cover"+ext), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644) //nolint:gosec // a cover is meant to be readable
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
