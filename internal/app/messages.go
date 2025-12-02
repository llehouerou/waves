// Package app contains application-level types and messages for the TUI.
package app

import (
	"time"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/navigator"
)

// TickMsg is sent periodically to update the UI (e.g., progress bar).
type TickMsg time.Time

// ScanResultMsg wraps navigator scan results for directory searching.
type ScanResultMsg navigator.ScanResult

// KeySequenceTimeoutMsg is sent when a key sequence times out
// (e.g., space key waiting for ff/lr suffix).
type KeySequenceTimeoutMsg struct{}

// TrackSkipTimeoutMsg is sent after debounce delay for track skip operations.
// The Version field is used to ignore stale timeouts when rapid key presses occur.
type TrackSkipTimeoutMsg struct {
	Version int
}

// LibraryScanProgressMsg wraps library scan progress updates.
type LibraryScanProgressMsg library.ScanProgress

// LibraryScanCompleteMsg is sent when library scanning finishes.
type LibraryScanCompleteMsg struct{}

// TrackFinishedMsg is sent when the current track finishes playing.
type TrackFinishedMsg struct{}

// FocusTarget represents which UI component has focus.
type FocusTarget int

const (
	// FocusNavigator indicates the file/library navigator has focus.
	FocusNavigator FocusTarget = iota
	// FocusQueue indicates the queue panel has focus.
	FocusQueue
)

// ViewMode represents the current navigator view type.
type ViewMode string

const (
	// ViewLibrary shows the music library browser.
	ViewLibrary ViewMode = "library"
	// ViewFileBrowser shows the filesystem browser.
	ViewFileBrowser ViewMode = "file"
	// ViewPlaylists shows the playlists browser.
	ViewPlaylists ViewMode = "playlists"
)

// QueueAction represents the type of queue operation to perform.
type QueueAction int

const (
	// QueueAddAndPlay adds tracks to queue and starts playing immediately.
	QueueAddAndPlay QueueAction = iota
	// QueueAdd adds tracks to queue without interrupting current playback.
	QueueAdd
	// QueueReplace clears the queue, adds tracks, and starts playing.
	QueueReplace
)

// InputMode represents the type of text input being collected.
type InputMode int

const (
	// InputNone indicates no text input is active.
	InputNone InputMode = iota
	// InputNewPlaylist indicates creating a new playlist.
	InputNewPlaylist
	// InputNewFolder indicates creating a new folder.
	InputNewFolder
	// InputRename indicates renaming a playlist or folder.
	InputRename
)

// PlaylistInputContext stores context for playlist operations.
type PlaylistInputContext struct {
	Mode     InputMode
	ItemID   int64  // For rename: ID of the item being renamed
	IsFolder bool   // For rename: whether item is a folder
	FolderID *int64 // Parent folder ID for creation
}

// AddToPlaylistContext stores tracks to add when user selects a playlist.
type AddToPlaylistContext struct {
	TrackIDs []int64 // Library track IDs to add
}

// DeleteConfirmContext stores context for delete confirmation.
type DeleteConfirmContext struct {
	ItemID   int64
	IsFolder bool
}
