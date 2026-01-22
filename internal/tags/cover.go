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
	data, mimeType, err = extractEmbeddedArt(path)
	if err != nil {
		return nil, "", err
	}
	if data != nil {
		return data, mimeType, nil
	}

	// Fall back to folder images
	return findFolderArt(filepath.Dir(path))
}

// extractEmbeddedArt reads embedded cover art from an audio file's metadata.
func extractEmbeddedArt(path string) (data []byte, mimeType string, err error) {
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

// findFolderArt looks for common cover art files in the given directory.
func findFolderArt(dir string) (data []byte, mimeType string, err error) {
	for _, filename := range coverArtFilenames {
		imgPath := filepath.Join(dir, filename)
		data, err := os.ReadFile(imgPath)
		if err != nil {
			// Try case-insensitive match
			imgPath = filepath.Join(dir, strings.ToUpper(filename))
			data, err = os.ReadFile(imgPath)
			if err != nil {
				continue
			}
		}

		// Determine MIME type from extension
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".png":
			mimeType = "image/png"
		default:
			mimeType = "application/octet-stream"
		}

		return data, mimeType, nil
	}

	return nil, "", nil
}
