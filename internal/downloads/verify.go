package downloads

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// trackNumberRegex matches track numbers at the start of filenames.
// Handles formats like: "01 - Song.flac", "1. Song.mp3", "01_Song.flac"
var trackNumberRegex = regexp.MustCompile(`^(\d+)`)

// ExtractFolderName extracts the last path component from a slskd directory path.
// Example: "@@user1\Music\Artist - Album" -> "Artist - Album"
func ExtractFolderName(slskdDir string) string {
	// Normalize path separators (slskd uses backslashes)
	normalized := strings.ReplaceAll(slskdDir, "\\", "/")
	// Remove leading @@ and username prefix if present
	normalized = strings.TrimPrefix(normalized, "@@")
	// Get the last component
	return filepath.Base(normalized)
}

// BuildDiskPath constructs the expected disk path for a download folder.
func BuildDiskPath(completedPath, slskdDir string) string {
	folderName := ExtractFolderName(slskdDir)
	return filepath.Join(completedPath, folderName)
}

// ParseTrackNumber extracts a track number from a filename.
// Returns 0 if no track number is found.
func ParseTrackNumber(filename string) int {
	// Normalize backslashes (slskd uses Windows paths) before getting base name
	normalized := strings.ReplaceAll(filename, "\\", "/")
	base := filepath.Base(normalized)
	matches := trackNumberRegex.FindStringSubmatch(base)
	if len(matches) < 2 {
		return 0
	}
	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return num
}

// VerifyResult contains the result of verifying a file on disk.
type VerifyResult struct {
	FileID       int64
	Filename     string
	Exists       bool
	SizeMatches  bool
	DiskSize     int64
	ExpectedSize int64
}

// VerifyFileOnDisk checks if a file exists on disk and matches the expected size.
func VerifyFileOnDisk(completedPath, slskdDir, filename string, expectedSize int64) VerifyResult {
	result := VerifyResult{
		Filename:     filename,
		ExpectedSize: expectedSize,
	}

	// Build full path to file
	folderPath := BuildDiskPath(completedPath, slskdDir)
	// Normalize backslashes (slskd uses Windows paths) before getting base name
	normalizedFilename := strings.ReplaceAll(filename, "\\", "/")
	filePath := filepath.Join(folderPath, filepath.Base(normalizedFilename))

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return result
	}

	result.Exists = true
	result.DiskSize = info.Size()
	result.SizeMatches = info.Size() == expectedSize

	return result
}

// VerifyDownloadFiles verifies all files in a download against disk.
// Returns a map of file ID to verification result.
func VerifyDownloadFiles(completedPath string, download *Download) map[int64]VerifyResult {
	results := make(map[int64]VerifyResult)

	for _, f := range download.Files {
		result := VerifyFileOnDisk(completedPath, download.SlskdDirectory, f.Filename, f.Size)
		result.FileID = f.ID
		results[f.ID] = result
	}

	return results
}

// SortFilesByTrackNumber sorts files by their parsed track number.
// Files without track numbers are sorted alphabetically at the end.
func SortFilesByTrackNumber(files []DownloadFile) []DownloadFile {
	sorted := make([]DownloadFile, len(files))
	copy(sorted, files)

	sort.Slice(sorted, func(i, j int) bool {
		numI := ParseTrackNumber(sorted[i].Filename)
		numJ := ParseTrackNumber(sorted[j].Filename)

		// Both have track numbers - sort by number
		if numI > 0 && numJ > 0 {
			return numI < numJ
		}
		// Only one has track number - it comes first
		if numI > 0 {
			return true
		}
		if numJ > 0 {
			return false
		}
		// Neither has track number - sort alphabetically
		return sorted[i].Filename < sorted[j].Filename
	})

	return sorted
}

// DeleteFilesFromDisk removes all files for a download from the completed folder.
// Also removes the download folder if it becomes empty.
func DeleteFilesFromDisk(completedPath string, download *Download) error {
	if completedPath == "" {
		return nil
	}

	folderPath := BuildDiskPath(completedPath, download.SlskdDirectory)

	// Check if folder exists
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		return nil // Nothing to delete
	}

	// Delete each file
	for _, f := range download.Files {
		normalizedFilename := strings.ReplaceAll(f.Filename, "\\", "/")
		filePath := filepath.Join(folderPath, filepath.Base(normalizedFilename))
		// Ignore errors - file might not exist
		_ = os.Remove(filePath)
	}

	// Try to remove the folder (will fail if not empty, which is fine)
	_ = os.Remove(folderPath)

	return nil
}
