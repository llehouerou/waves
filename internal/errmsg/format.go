// Package errmsg provides consistent error formatting for user-facing messages.
package errmsg

import "fmt"

// Op represents an operation that can fail.
type Op string

// Operation constants - grouped by domain.
const (
	// Library operations
	OpLibraryDelete  Op = "delete track from library"
	OpLibraryScan    Op = "scan library"
	OpLibraryLoad    Op = "load library"
	OpLibraryRebuild Op = "rebuild library index"

	// Source operations
	OpSourceAdd    Op = "add library source"
	OpSourceRemove Op = "remove library source"
	OpSourceLoad   Op = "load library sources"

	// Download operations
	OpDownloadQueue   Op = "queue download"
	OpDownloadDelete  Op = "delete download"
	OpDownloadClear   Op = "clear completed downloads"
	OpDownloadRefresh Op = "refresh downloads"

	// Import operations
	OpImportFile Op = "import file"
	OpImportTags Op = "read file tags"

	// Playlist operations
	OpPlaylistCreate   Op = "create playlist"
	OpPlaylistRename   Op = "rename playlist"
	OpPlaylistDelete   Op = "delete playlist"
	OpPlaylistAddTrack Op = "add track to playlist"
	OpPlaylistRemove   Op = "remove track from playlist"
	OpPlaylistMove     Op = "move playlist item"

	// Folder operations
	OpFolderCreate Op = "create folder"
	OpFolderRename Op = "rename folder"
	OpFolderDelete Op = "delete folder"

	// Queue operations
	OpQueueLoad Op = "load queue"
	OpQueueSave Op = "save queue"
	OpQueueAdd  Op = "add to queue"

	// Playback operations
	OpPlaybackStart Op = "start playback"
	OpPlaybackSeek  Op = "seek"

	// Favorites
	OpFavoriteToggle Op = "update favorites"

	// File operations
	OpFileDelete Op = "delete file"
	OpFileLoad   Op = "load file"

	// Album view
	OpAlbumLoad    Op = "load albums"
	OpPresetLoad   Op = "load album presets"
	OpPresetSave   Op = "save album preset"
	OpPresetDelete Op = "delete album preset"

	// Initialization
	OpInitialize Op = "initialize application"
)

// Format creates a user-friendly error message.
func Format(op Op, err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("Failed to %s: %v", op, err)
}

// FormatWith creates an error message with additional context.
func FormatWith(op Op, context string, err error) string {
	if err == nil {
		return ""
	}
	if context == "" {
		return Format(op, err)
	}
	return fmt.Sprintf("Failed to %s '%s': %v", op, context, err)
}
