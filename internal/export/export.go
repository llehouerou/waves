package export

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Exporter handles copying and converting files for export.
type Exporter struct{}

// NewExporter creates a new Exporter.
func NewExporter() *Exporter {
	return &Exporter{}
}

// NeedsConversion returns true if the file extension requires conversion.
func NeedsConversion(ext string) bool {
	return strings.EqualFold(ext, ".flac")
}

// CopyFile copies a file from src to dst.
// Creates parent directories if needed.
// Skips if destination already exists.
func (e *Exporter) CopyFile(src, dst string) error {
	// Check if destination exists (skip if so)
	if _, err := os.Stat(dst); err == nil {
		return nil // Already exists, skip
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Open source
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	// Create destination
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dstFile.Close()

	// Copy content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		// Clean up partial file
		os.Remove(dst)
		return fmt.Errorf("copy: %w", err)
	}

	return dstFile.Close()
}

// ConvertToMP3 converts a FLAC file to MP3 using ffmpeg.
// Uses 320kbps CBR preset.
func (e *Exporter) ConvertToMP3(src, dst string) error {
	// Check if destination exists
	if _, err := os.Stat(dst); err == nil {
		return nil // Already exists, skip
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Run ffmpeg
	cmd := exec.Command("ffmpeg",
		"-i", src,
		"-codec:a", "libmp3lame",
		"-b:a", "320k",
		"-map_metadata", "0", // Preserve tags
		"-id3v2_version", "3",
		"-y", // Overwrite temp files
		dst,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up partial file
		os.Remove(dst)
		return fmt.Errorf("ffmpeg conversion failed: %w\n%s", err, string(output))
	}

	return nil
}

// ExportFile exports a single track, converting if needed.
func (e *Exporter) ExportFile(src, dst string, convert bool) error {
	ext := strings.ToLower(filepath.Ext(src))

	if convert && NeedsConversion(ext) {
		// Change extension to .mp3
		dst = strings.TrimSuffix(dst, filepath.Ext(dst)) + ".mp3"
		return e.ConvertToMP3(src, dst)
	}

	return e.CopyFile(src, dst)
}
