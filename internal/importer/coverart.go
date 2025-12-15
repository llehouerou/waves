package importer

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Common cover art filenames (case-insensitive)
var coverArtNames = []string{
	"cover",
	"folder",
	"front",
	"album",
	"albumart",
	"artwork",
}

// Supported image extensions
var imageExtensions = []string{".jpg", ".jpeg", ".png"}

// FindCoverArt looks for a cover art image file in the given directory.
// Returns the full path to the cover art file, or empty string if not found.
func FindCoverArt(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		baseName := strings.ToLower(strings.TrimSuffix(name, ext))

		// Check if it's an image file
		if !slices.Contains(imageExtensions, ext) {
			continue
		}

		// Check if filename matches common cover art names
		if slices.Contains(coverArtNames, baseName) {
			return filepath.Join(dir, name)
		}
	}

	return ""
}

// ImportCoverArt moves a cover art file to the destination album directory.
// The cover art is renamed to "cover.<ext>" in the destination.
// Returns the destination path, or empty string if no cover art was found/moved.
func ImportCoverArt(sourceDir, destDir string, copyMode bool) (string, error) {
	// Find cover art in source directory
	coverPath := FindCoverArt(sourceDir)
	if coverPath == "" {
		return "", nil // No cover art found, not an error
	}

	// Determine destination path (keep original extension)
	ext := strings.ToLower(filepath.Ext(coverPath))
	destPath := filepath.Join(destDir, "cover"+ext)

	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		// Cover already exists at destination, skip
		return destPath, nil
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	// Copy or move the file
	if copyMode {
		if err := copyFile(coverPath, destPath); err != nil {
			return "", err
		}
	} else {
		if err := moveFile(coverPath, destPath); err != nil {
			return "", err
		}
	}

	return destPath, nil
}
