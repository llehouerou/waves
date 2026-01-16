package download

import (
	"sort"
	"strings"

	"github.com/llehouerou/waves/internal/slskd"
)

// FilterStats tracks how many results were filtered out and why.
type FilterStats struct {
	NoFreeSlot      int // Directories filtered out (user has no free upload slot)
	NoAudioFiles    int // Directories with no audio files at all
	WrongFormat     int // Directories filtered by format (has audio but wrong format)
	WrongTrackCount int // Directories with wrong track count
	TotalResponses  int // Total user responses received
	TotalDirs       int // Total directories examined
	ExpectedTracks  int // The expected track count
}

// FilterOptions controls which filters are applied to search results.
type FilterOptions struct {
	Format           FormatFilter // Lossy/Lossless/Both
	FilterNoSlot     bool         // Filter out users with no free slot
	FilterTrackCount bool         // Filter by track count
	ExpectedTracks   int          // Expected track count from MusicBrainz (0 = auto-detect)
	ReleaseYear      string       // Release year for folder name matching (e.g., "1980")
}

// dirEntry represents a single directory from a user's search response.
type dirEntry struct {
	Username    string
	Directory   string
	Files       []slskd.File
	HasFreeSlot bool
	UploadSpeed int
}

// FilterAndScoreResults processes slskd search responses, groups files by directory,
// filters by format and track count, and returns results sorted by upload speed.
func FilterAndScoreResults(responses []slskd.SearchResponse, opts FilterOptions) ([]SlskdResult, FilterStats) {
	var stats FilterStats

	stats.TotalResponses = len(responses)

	// First, extract all directories from all responses
	// Estimate capacity: assume average of 2 directories per response
	allDirs := make([]dirEntry, 0, len(responses)*2)
	for i := range responses {
		resp := &responses[i]
		dirFiles := groupFilesByDirectory(resp.Files)
		for dir, files := range dirFiles {
			allDirs = append(allDirs, dirEntry{
				Username:    resp.Username,
				Directory:   dir,
				Files:       files,
				HasFreeSlot: resp.HasFreeSlot,
				UploadSpeed: resp.UploadSpeed,
			})
		}
	}

	stats.TotalDirs = len(allDirs)

	// Pre-allocate candidates (may be smaller after filtering)
	candidates := make([]SlskdResult, 0, len(allDirs))

	// Process each directory independently
	for _, d := range allDirs {
		// Filter directories from users with no free upload slots (if enabled)
		if opts.FilterNoSlot && !d.HasFreeSlot {
			stats.NoFreeSlot++
			continue
		}

		// Check if directory has any audio files at all
		hasAnyAudio := hasAnyAudioFiles(d.Files)
		if !hasAnyAudio {
			stats.NoAudioFiles++
			continue
		}

		// Extract audio files based on format filter
		audioFiles, format := extractAudioFilesWithFilter(d.Files, opts.Format)
		if len(audioFiles) == 0 {
			// Has audio but wrong format
			stats.WrongFormat++
			continue
		}

		result := SlskdResult{
			Username:    d.Username,
			Directory:   d.Directory,
			Files:       audioFiles,
			FileCount:   len(audioFiles),
			Format:      format,
			BitRate:     getMostCommonBitRate(audioFiles),
			UploadSpeed: d.UploadSpeed,
		}

		// Calculate total size
		for _, f := range audioFiles {
			result.TotalSize += f.Size
		}

		candidates = append(candidates, result)
	}

	// Determine expected track count: use MB value if provided, otherwise auto-detect
	expectedTracks := opts.ExpectedTracks
	if expectedTracks == 0 {
		// Fall back to finding most common track count among results
		expectedTracks = findExpectedTrackCount(candidates)
	}
	stats.ExpectedTracks = expectedTracks

	// Filter out results that don't match exact track count (if enabled)
	results := make([]SlskdResult, 0, len(candidates))
	for i := range candidates {
		r := &candidates[i]
		if opts.FilterTrackCount && expectedTracks > 0 && r.FileCount != expectedTracks {
			stats.WrongTrackCount++
			continue
		}
		// Check if folder name contains the release year (for prioritization)
		if opts.ReleaseYear != "" && strings.Contains(r.Directory, opts.ReleaseYear) {
			r.IsComplete = true // Reuse this field to indicate year match
		}
		results = append(results, *r)
	}

	// Sort by: year match first, then upload speed descending
	sort.Slice(results, func(i, j int) bool {
		// Prioritize results with year match in folder name
		if results[i].IsComplete != results[j].IsComplete {
			return results[i].IsComplete
		}
		return results[i].UploadSpeed > results[j].UploadSpeed
	})

	return results, stats
}

// extractAudioFilesWithFilter extracts audio files based on format filter.
// Returns the files and the format string.
func extractAudioFilesWithFilter(files []slskd.File, format FormatFilter) (audioFiles []slskd.File, formatStr string) {
	switch format {
	case FormatLossless:
		lossless, formatName := filterLosslessFiles(files)
		if len(lossless) > 0 {
			return lossless, formatName
		}
		return nil, ""
	case FormatLossy:
		lossy, formatName := filterLossyFiles(files)
		if len(lossy) > 0 {
			return lossy, formatName
		}
		return nil, ""
	case FormatBoth:
		// Prefer lossless, fall back to lossy
		lossless, formatName := filterLosslessFiles(files)
		if len(lossless) > 0 {
			return lossless, formatName
		}
		lossy, formatName := filterLossyFiles(files)
		if len(lossy) > 0 {
			return lossy, formatName
		}
		return nil, ""
	}
	return nil, ""
}

// findExpectedTrackCount finds the most common track count among results.
// Returns 0 if no clear majority (need at least 3 results with same count).
func findExpectedTrackCount(results []SlskdResult) int {
	if len(results) == 0 {
		return 0
	}

	// Count occurrences of each track count
	counts := make(map[int]int)
	for _, r := range results {
		counts[r.FileCount]++
	}

	// Find the most common count
	var maxCount, maxTrackCount int
	for trackCount, count := range counts {
		if count > maxCount {
			maxCount = count
			maxTrackCount = trackCount
		}
	}

	// Need at least 3 results with the same count to consider it "expected"
	if maxCount >= 3 {
		return maxTrackCount
	}

	return 0
}

// losslessExtensions maps lossless audio extensions to display names.
var losslessExtensions = map[string]string{
	"flac": "FLAC",
	"wav":  "WAV",
	"aiff": "AIFF",
	"aif":  "AIFF",
	"alac": "ALAC",
	"ape":  "APE",
	"wv":   "WavPack",
	"tta":  "TTA",
}

// lossyExtensions maps lossy audio extensions to display names.
var lossyExtensions = map[string]string{
	"mp3":  "MP3",
	"m4a":  "AAC",
	"aac":  "AAC",
	"ogg":  "OGG",
	"opus": "Opus",
	"wma":  "WMA",
	"mpc":  "Musepack",
}

// filterLosslessFiles returns only lossless audio files.
// Returns the files and the format name of the most common format found.
func filterLosslessFiles(files []slskd.File) (result []slskd.File, formatName string) {
	formatCounts := make(map[string]int)

	for _, f := range files {
		ext := getFileExtension(f)
		if name, ok := losslessExtensions[ext]; ok {
			result = append(result, f)
			formatCounts[name]++
		}
	}

	// Find most common format for display
	formatName = getMostCommonFormat(formatCounts)
	return result, formatName
}

// filterLossyFiles returns only lossy audio files.
// Returns the files and the format name of the most common format found.
func filterLossyFiles(files []slskd.File) (result []slskd.File, formatName string) {
	formatCounts := make(map[string]int)

	for _, f := range files {
		ext := getFileExtension(f)
		if name, ok := lossyExtensions[ext]; ok {
			result = append(result, f)
			formatCounts[name]++
		}
	}

	// Find most common format for display
	formatName = getMostCommonFormat(formatCounts)
	return result, formatName
}

// getMostCommonFormat returns the most common format name from counts.
func getMostCommonFormat(counts map[string]int) string {
	var maxCount int
	var maxFormat string
	for format, count := range counts {
		if count > maxCount {
			maxCount = count
			maxFormat = format
		}
	}
	return maxFormat
}

// getMostCommonBitRate returns the most common bitrate among files (in kbps).
// Returns 0 if no bitrate info is available.
func getMostCommonBitRate(files []slskd.File) int {
	counts := make(map[int]int)
	for _, f := range files {
		if f.BitRate > 0 {
			counts[f.BitRate]++
		}
	}

	var maxCount, maxBitRate int
	for bitrate, count := range counts {
		if count > maxCount {
			maxCount = count
			maxBitRate = bitrate
		}
	}
	return maxBitRate
}

// groupFilesByDirectory groups files by their parent directory.
// Handles both Unix (/) and Windows (\) path separators since slskd
// returns paths from various operating systems.
func groupFilesByDirectory(files []slskd.File) map[string][]slskd.File {
	groups := make(map[string][]slskd.File)
	for _, f := range files {
		dir := getParentDirectory(f.Filename)
		groups[dir] = append(groups[dir], f)
	}
	return groups
}

// getParentDirectory extracts the parent directory from a path.
// Handles both Unix (/) and Windows (\) path separators.
func getParentDirectory(path string) string {
	// Find the last separator (either / or \)
	lastSlash := strings.LastIndex(path, "/")
	lastBackslash := strings.LastIndex(path, "\\")

	// Use whichever is later in the string
	lastSep := max(lastSlash, lastBackslash)

	if lastSep <= 0 {
		return "."
	}
	return path[:lastSep]
}

// getFileExtension returns the lowercase extension without dot.
// Falls back to extracting from filename if Extension field is empty.
func getFileExtension(f slskd.File) string {
	ext := strings.ToLower(strings.TrimPrefix(f.Extension, "."))
	if ext != "" {
		return ext
	}
	// Extract from filename
	if idx := strings.LastIndex(f.Filename, "."); idx != -1 {
		return strings.ToLower(f.Filename[idx+1:])
	}
	return ""
}

// hasAnyAudioFiles checks if a directory contains any audio files (lossless or lossy).
func hasAnyAudioFiles(files []slskd.File) bool {
	for _, f := range files {
		ext := getFileExtension(f)
		if _, ok := losslessExtensions[ext]; ok {
			return true
		}
		if _, ok := lossyExtensions[ext]; ok {
			return true
		}
	}
	return false
}
