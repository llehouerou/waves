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

// discFolderRegex matches a folder named after a disc: "CD1", "CD 2", "CD02",
// "Disc 1", "Disk 2", optionally followed by a title after a separator
// ("CD1 - Live", "Disc 2 (Bonus)").
var discFolderRegex = regexp.MustCompile(`(?i)^(?:cd|dis[ck])\s*(\d+)(?:\s*[-_.:(\[].*)?$`)

// DiscNumber is the disc a folder name means ("CD2", "Disc 2 - Live"), or 0
// when it doesn't name a disc.
func DiscNumber(folderName string) int {
	matches := discFolderRegex.FindStringSubmatch(strings.TrimSpace(folderName))
	if matches == nil {
		return 0
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0 // too many digits to be a disc
	}
	return n
}

// SlskdFolder is the slskd folder a file is in: its path up to the last
// separator, either / or \ since slskd paths come from any OS. "." when the
// path has no folder.
func SlskdFolder(path string) string {
	lastSep := max(strings.LastIndex(path, "/"), strings.LastIndex(path, "\\"))
	if lastSep <= 0 {
		return "."
	}
	return path[:lastSep]
}

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

// BuildDiskPath is the download folder slskd writes the files of slskd folder
// slskdDir to.
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

// ExpectedDiskPath returns where slskd is expected to place the completed file
// at slskd path filename: <completedPath>/<last component of its folder>/<base filename>.
func ExpectedDiskPath(completedPath, filename string) string {
	// Normalize backslashes (slskd uses Windows paths) before getting base name
	normalized := strings.ReplaceAll(filename, "\\", "/")
	return filepath.Join(BuildDiskPath(completedPath, SlskdFolder(filename)), filepath.Base(normalized))
}

// VerifyFileOnDisk checks if a file exists on disk and matches the expected size.
func VerifyFileOnDisk(completedPath, filename string, expectedSize int64) VerifyResult {
	result := VerifyResult{
		Filename:     filename,
		ExpectedSize: expectedSize,
	}

	info, err := os.Stat(ExpectedDiskPath(completedPath, filename))
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
		result := VerifyFileOnDisk(completedPath, f.Filename, f.Size)
		result.FileID = f.ID
		results[f.ID] = result
	}

	return results
}

// SortFilesByTrackNumber sorts files by folder, disc folders in disc order,
// then by their parsed track number within a folder. Files without track
// numbers are sorted alphabetically at the end of their folder.
func SortFilesByTrackNumber(files []DownloadFile) []DownloadFile {
	sorted := make([]DownloadFile, len(files))
	copy(sorted, files)

	sort.SliceStable(sorted, func(i, j int) bool {
		if dirI, dirJ := SlskdFolder(sorted[i].Filename), SlskdFolder(sorted[j].Filename); dirI != dirJ {
			discI, discJ := DiscNumber(ExtractFolderName(dirI)), DiscNumber(ExtractFolderName(dirJ))
			if discI != discJ {
				return discI < discJ
			}
			return dirI < dirJ
		}

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
