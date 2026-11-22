package tags

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
)

// Common cover art filenames to look for in album folders.
var coverArtFilenames = []string{
	"cover.jpg", "cover.jpeg", "cover.png",
	"folder.jpg", "folder.jpeg", "folder.png",
	"album.jpg", "album.jpeg", "album.png",
	"front.jpg", "front.jpeg", "front.png",
	"artwork.jpg", "artwork.jpeg", "artwork.png",
}

// ExtractCoverArt reads cover art for an audio file.
// It first tries to extract embedded art from the file metadata.
// If no embedded art is found, it looks for common cover image files
// in the same directory (cover.jpg, folder.jpg, album.png, etc.).
// Returns the image data and MIME type, or nil if no art is found.
func ExtractCoverArt(path string) (data []byte, mimeType string, err error) {
	// Try embedded art first
	data, mimeType, err = ExtractEmbeddedArt(path)
	if err != nil {
		return nil, "", err
	}
	if data != nil {
		return data, mimeType, nil
	}

	// Fall back to folder images
	return findFolderArt(filepath.Dir(path))
}

// ExtractEmbeddedArt reads embedded cover art from an audio file's metadata.
func ExtractEmbeddedArt(path string) (data []byte, mimeType string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil, "", err
	}

	pic := m.Picture()
	if pic == nil {
		return nil, "", nil
	}

	return pic.Data, pic.MIMEType, nil
}

// HasFolderArt reports whether dir holds one of the cover image files that
// ExtractCoverArt falls back to, whatever its case.
func HasFolderArt(dir string) bool {
	return len(folderArtFiles(dir)) > 0
}

// findFolderArt reads the first of the common cover art files found in the
// given directory, whatever the case of its name.
func findFolderArt(dir string) (data []byte, mimeType string, err error) {
	for _, path := range folderArtFiles(dir) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return data, folderArtMIME(path), nil
	}

	return nil, "", nil
}

// folderArtFiles lists the paths of the common cover art files in dir,
// matched whatever their case, in the order of coverArtFilenames.
func folderArtFiles(dir string) []string {
	// A folder that can't be read has no cover file to give either.
	entries, _ := os.ReadDir(dir) //nolint:errcheck // see above
	var paths []string
	for _, filename := range coverArtFilenames {
		for _, e := range entries {
			if strings.EqualFold(e.Name(), filename) {
				paths = append(paths, filepath.Join(dir, e.Name()))
			}
		}
	}
	return paths
}

// folderArtMIME is the MIME type of a cover art file, from its extension.
func folderArtMIME(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}
