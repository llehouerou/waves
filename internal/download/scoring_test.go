package download

import (
	"testing"

	"github.com/llehouerou/waves/internal/slskd"
)

const (
	formatFLAC = "FLAC"
	formatMP3  = "MP3"
)

func TestGetParentDirectory(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{`C:\Users\Music\Artist\Album\track.mp3`, `C:\Users\Music\Artist\Album`},
		{"/home/user/music/track.flac", "/home/user/music"},
		{`Album\track.mp3`, "Album"},
		{"track.mp3", "."},
		{"", "."},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := getParentDirectory(tt.path)
			if got != tt.want {
				t.Errorf("getParentDirectory(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		name string
		file slskd.File
		want string
	}{
		{
			name: "extension field set",
			file: slskd.File{Extension: ".FLAC", Filename: "track.mp3"},
			want: "flac",
		},
		{
			name: "extension from filename",
			file: slskd.File{Extension: "", Filename: "track.mp3"},
			want: "mp3",
		},
		{
			name: "extension with dot prefix",
			file: slskd.File{Extension: "wav", Filename: "track.wav"},
			want: "wav",
		},
		{
			name: "no extension",
			file: slskd.File{Extension: "", Filename: "noext"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getFileExtension(tt.file)
			if got != tt.want {
				t.Errorf("getFileExtension() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasAnyAudioFiles(t *testing.T) {
	tests := []struct {
		name  string
		files []slskd.File
		want  bool
	}{
		{
			name:  "empty",
			files: nil,
			want:  false,
		},
		{
			name: "has flac",
			files: []slskd.File{
				{Extension: ".flac"},
			},
			want: true,
		},
		{
			name: "has mp3",
			files: []slskd.File{
				{Extension: ".mp3"},
			},
			want: true,
		},
		{
			name: "only non-audio",
			files: []slskd.File{
				{Extension: ".jpg"},
				{Extension: ".txt"},
			},
			want: false,
		},
		{
			name: "mixed with audio",
			files: []slskd.File{
				{Extension: ".jpg"},
				{Extension: ".flac"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAnyAudioFiles(tt.files)
			if got != tt.want {
				t.Errorf("hasAnyAudioFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupFilesByDirectory(t *testing.T) {
	files := []slskd.File{
		{Filename: `Album1\01 - Track1.flac`},
		{Filename: `Album1\02 - Track2.flac`},
		{Filename: `Album2\01 - Track1.mp3`},
		{Filename: "single.mp3"},
	}

	groups := groupFilesByDirectory(files)

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	if len(groups["Album1"]) != 2 {
		t.Errorf("Album1 should have 2 files, got %d", len(groups["Album1"]))
	}

	if len(groups["Album2"]) != 1 {
		t.Errorf("Album2 should have 1 file, got %d", len(groups["Album2"]))
	}

	if len(groups["."]) != 1 {
		t.Errorf(". should have 1 file, got %d", len(groups["."]))
	}
}

func TestFindExpectedTrackCount(t *testing.T) {
	tests := []struct {
		name    string
		results []SlskdResult
		want    int
	}{
		{
			name:    "empty",
			results: nil,
			want:    0,
		},
		{
			name: "less than 3 results",
			results: []SlskdResult{
				{FileCount: 10},
				{FileCount: 10},
			},
			want: 0,
		},
		{
			name: "3 matching results",
			results: []SlskdResult{
				{FileCount: 12},
				{FileCount: 12},
				{FileCount: 12},
			},
			want: 12,
		},
		{
			name: "majority wins",
			results: []SlskdResult{
				{FileCount: 10},
				{FileCount: 12},
				{FileCount: 12},
				{FileCount: 12},
				{FileCount: 15},
			},
			want: 12,
		},
		{
			name: "no clear majority",
			results: []SlskdResult{
				{FileCount: 10},
				{FileCount: 11},
				{FileCount: 12},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findExpectedTrackCount(tt.results)
			if got != tt.want {
				t.Errorf("findExpectedTrackCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFilterLosslessFiles(t *testing.T) {
	files := []slskd.File{
		{Extension: ".flac", Filename: "01.flac"},
		{Extension: ".flac", Filename: "02.flac"},
		{Extension: ".mp3", Filename: "03.mp3"},
		{Extension: ".jpg", Filename: "cover.jpg"},
	}

	result, format := filterLosslessFiles(files)

	if len(result) != 2 {
		t.Errorf("expected 2 lossless files, got %d", len(result))
	}

	if format != formatFLAC {
		t.Errorf("expected format FLAC, got %s", format)
	}
}

func TestFilterLossyFiles(t *testing.T) {
	files := []slskd.File{
		{Extension: ".mp3", Filename: "01.mp3"},
		{Extension: ".mp3", Filename: "02.mp3"},
		{Extension: ".flac", Filename: "03.flac"},
		{Extension: ".jpg", Filename: "cover.jpg"},
	}

	result, format := filterLossyFiles(files)

	if len(result) != 2 {
		t.Errorf("expected 2 lossy files, got %d", len(result))
	}

	if format != formatMP3 {
		t.Errorf("expected format MP3, got %s", format)
	}
}

func TestExtractAudioFilesWithFilter(t *testing.T) {
	files := []slskd.File{
		{Extension: ".flac", Filename: "01.flac"},
		{Extension: ".mp3", Filename: "02.mp3"},
	}

	t.Run("lossless filter", func(t *testing.T) {
		result, format := extractAudioFilesWithFilter(files, FormatLossless)
		if len(result) != 1 {
			t.Errorf("expected 1 file, got %d", len(result))
		}
		if format != formatFLAC {
			t.Errorf("expected format FLAC, got %s", format)
		}
	})

	t.Run("lossy filter", func(t *testing.T) {
		result, format := extractAudioFilesWithFilter(files, FormatLossy)
		if len(result) != 1 {
			t.Errorf("expected 1 file, got %d", len(result))
		}
		if format != formatMP3 {
			t.Errorf("expected format MP3, got %s", format)
		}
	})

	t.Run("both filter prefers lossless", func(t *testing.T) {
		result, format := extractAudioFilesWithFilter(files, FormatBoth)
		if len(result) != 1 {
			t.Errorf("expected 1 file, got %d", len(result))
		}
		if format != formatFLAC {
			t.Errorf("expected format FLAC, got %s", format)
		}
	})

	t.Run("both filter falls back to lossy", func(t *testing.T) {
		lossyOnly := []slskd.File{{Extension: ".mp3"}}
		result, format := extractAudioFilesWithFilter(lossyOnly, FormatBoth)
		if len(result) != 1 {
			t.Errorf("expected 1 file, got %d", len(result))
		}
		if format != formatMP3 {
			t.Errorf("expected format MP3, got %s", format)
		}
	})
}

func TestGetMostCommonBitRate(t *testing.T) {
	tests := []struct {
		name  string
		files []slskd.File
		want  int
	}{
		{
			name:  "empty",
			files: nil,
			want:  0,
		},
		{
			name: "no bitrate info",
			files: []slskd.File{
				{BitRate: 0},
				{BitRate: 0},
			},
			want: 0,
		},
		{
			name: "single bitrate",
			files: []slskd.File{
				{BitRate: 320},
				{BitRate: 320},
			},
			want: 320,
		},
		{
			name: "majority wins",
			files: []slskd.File{
				{BitRate: 320},
				{BitRate: 320},
				{BitRate: 192},
			},
			want: 320,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMostCommonBitRate(tt.files)
			if got != tt.want {
				t.Errorf("getMostCommonBitRate() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFilterAndScoreResults(t *testing.T) {
	responses := []slskd.SearchResponse{
		{
			Username:    "user1",
			HasFreeSlot: true,
			UploadSpeed: 1000,
			Files: []slskd.File{
				{Filename: `Album\01 - Track1.flac`, Extension: ".flac", Size: 30000000},
				{Filename: `Album\02 - Track2.flac`, Extension: ".flac", Size: 30000000},
			},
		},
		{
			Username:    "user2",
			HasFreeSlot: true,
			UploadSpeed: 500,
			Files: []slskd.File{
				{Filename: `Album\01 - Track1.mp3`, Extension: ".mp3", Size: 5000000},
				{Filename: `Album\02 - Track2.mp3`, Extension: ".mp3", Size: 5000000},
			},
		},
		{
			Username:    "user3",
			HasFreeSlot: false,
			UploadSpeed: 2000,
			Files: []slskd.File{
				{Filename: `Album\01 - Track1.flac`, Extension: ".flac", Size: 30000000},
			},
		},
	}

	t.Run("filter no slot", func(t *testing.T) {
		opts := FilterOptions{
			Format:       FormatBoth,
			FilterNoSlot: true,
		}
		results, stats := FilterAndScoreResults(responses, opts)

		// user3 should be filtered out
		if stats.NoFreeSlot != 1 {
			t.Errorf("expected 1 NoFreeSlot, got %d", stats.NoFreeSlot)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("lossless only filter", func(t *testing.T) {
		opts := FilterOptions{
			Format:       FormatLossless,
			FilterNoSlot: false,
		}
		results, stats := FilterAndScoreResults(responses, opts)

		// user2 (MP3) should be filtered out by format
		if stats.WrongFormat != 1 {
			t.Errorf("expected 1 WrongFormat, got %d", stats.WrongFormat)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results (FLAC only), got %d", len(results))
		}
	})

	t.Run("sorted by upload speed", func(t *testing.T) {
		opts := FilterOptions{
			Format:       FormatBoth,
			FilterNoSlot: false,
		}
		results, _ := FilterAndScoreResults(responses, opts)

		if len(results) < 2 {
			t.Fatalf("expected at least 2 results")
		}

		// Results should be sorted by upload speed descending
		if results[0].UploadSpeed < results[1].UploadSpeed {
			t.Errorf("results not sorted by upload speed: %d < %d",
				results[0].UploadSpeed, results[1].UploadSpeed)
		}
	})
}
