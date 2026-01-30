package tags

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Write writes tag metadata to a music file.
// The file must already exist. This operation modifies the file in place.
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
