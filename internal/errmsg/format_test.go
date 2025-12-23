//nolint:goconst // test cases intentionally repeat strings for readability
package errmsg

import (
	"errors"
	"testing"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		op       Op
		err      error
		expected string
	}{
		{
			name:     "nil error returns empty string",
			op:       OpLibraryDelete,
			err:      nil,
			expected: "",
		},
		{
			name:     "formats error with operation",
			op:       OpLibraryDelete,
			err:      errors.New("file not found"),
			expected: "Failed to delete track from library: file not found",
		},
		{
			name:     "library scan operation",
			op:       OpLibraryScan,
			err:      errors.New("permission denied"),
			expected: "Failed to scan library: permission denied",
		},
		{
			name:     "download operation",
			op:       OpDownloadQueue,
			err:      errors.New("network error"),
			expected: "Failed to queue download: network error",
		},
		{
			name:     "playlist operation",
			op:       OpPlaylistCreate,
			err:      errors.New("already exists"),
			expected: "Failed to create playlist: already exists",
		},
		{
			name:     "playback operation",
			op:       OpPlaybackStart,
			err:      errors.New("no audio device"),
			expected: "Failed to start playback: no audio device",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Format(tt.op, tt.err)
			if result != tt.expected {
				t.Errorf("Format(%q, %v) = %q, want %q", tt.op, tt.err, result, tt.expected)
			}
		})
	}
}

func TestFormatWith(t *testing.T) {
	tests := []struct {
		name     string
		op       Op
		context  string
		err      error
		expected string
	}{
		{
			name:     "nil error returns empty string",
			op:       OpFileDelete,
			context:  "song.mp3",
			err:      nil,
			expected: "",
		},
		{
			name:     "formats error with context",
			op:       OpFileDelete,
			context:  "song.mp3",
			err:      errors.New("permission denied"),
			expected: "Failed to delete file 'song.mp3': permission denied",
		},
		{
			name:     "empty context falls back to Format",
			op:       OpFileDelete,
			context:  "",
			err:      errors.New("permission denied"),
			expected: "Failed to delete file: permission denied",
		},
		{
			name:     "playlist add track with context",
			op:       OpPlaylistAddTrack,
			context:  "My Playlist",
			err:      errors.New("track not found"),
			expected: "Failed to add track to playlist 'My Playlist': track not found",
		},
		{
			name:     "source add with path context",
			op:       OpSourceAdd,
			context:  "/home/user/music",
			err:      errors.New("directory not found"),
			expected: "Failed to add library source '/home/user/music': directory not found",
		},
		{
			name:     "import with filename context",
			op:       OpImportFile,
			context:  "album.flac",
			err:      errors.New("unsupported format"),
			expected: "Failed to import file 'album.flac': unsupported format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatWith(tt.op, tt.context, tt.err)
			if result != tt.expected {
				t.Errorf("FormatWith(%q, %q, %v) = %q, want %q", tt.op, tt.context, tt.err, result, tt.expected)
			}
		})
	}
}

func TestOpConstants(t *testing.T) {
	// Verify that Op constants are non-empty and produce valid messages
	ops := []Op{
		OpLibraryDelete, OpLibraryScan, OpLibraryLoad, OpLibraryRebuild,
		OpSourceAdd, OpSourceRemove, OpSourceLoad,
		OpDownloadQueue, OpDownloadDelete, OpDownloadClear, OpDownloadRefresh,
		OpImportFile, OpImportTags,
		OpPlaylistCreate, OpPlaylistRename, OpPlaylistDelete,
		OpPlaylistAddTrack, OpPlaylistRemove, OpPlaylistMove,
		OpFolderCreate, OpFolderRename, OpFolderDelete,
		OpQueueLoad, OpQueueSave, OpQueueAdd,
		OpPlaybackStart, OpPlaybackSeek,
		OpFavoriteToggle,
		OpFileDelete, OpFileLoad,
		OpAlbumLoad, OpPresetLoad, OpPresetSave, OpPresetDelete,
		OpInitialize,
		OpLastfmAuth, OpLastfmScrobble, OpLastfmNowPlaying,
		OpRadioFill,
		OpExportFile, OpExportConvert, OpExportTarget, OpTargetDelete, OpTargetRename, OpVolumeDetect,
	}

	testErr := errors.New("test error")

	for _, op := range ops {
		t.Run(string(op), func(t *testing.T) {
			if op == "" {
				t.Error("Op constant should not be empty")
			}

			result := Format(op, testErr)
			if result == "" {
				t.Error("Format should return non-empty string for non-nil error")
			}

			// Verify the format includes the operation
			expected := "Failed to " + string(op) + ": test error"
			if result != expected {
				t.Errorf("Format = %q, want %q", result, expected)
			}
		})
	}
}
