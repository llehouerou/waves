// Package app contains application-level types and messages for the TUI.
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/navigator"
)

// Message category interfaces for type-based routing in Update().
// External messages (from other packages) cannot implement these interfaces,
// so they are handled separately in the Update() switch.

// PlaybackMessage is implemented by messages related to audio playback.
type PlaybackMessage interface {
	tea.Msg
	playbackMessage()
}

// NavigationMessage is implemented by messages related to navigation state.
type NavigationMessage interface {
	tea.Msg
	navigationMessage()
}

// InputMessage is implemented by messages related to user input handling.
type InputMessage interface {
	tea.Msg
	inputMessage()
}

// LoadingMessage is implemented by messages related to app initialization/loading.
type LoadingMessage interface {
	tea.Msg
	loadingMessage()
}

// LibraryScanMessage is implemented by messages related to library scanning.
type LibraryScanMessage interface {
	tea.Msg
	libraryScanMessage()
}

// TickMsg is sent periodically to update the UI (e.g., progress bar).
type TickMsg time.Time

func (TickMsg) playbackMessage() {}

// ScanResultMsg wraps navigator scan results for directory searching.
type ScanResultMsg navigator.ScanResult

func (ScanResultMsg) navigationMessage() {}

// KeySequenceTimeoutMsg is sent when a key sequence times out
// (e.g., space key waiting for ff/lr suffix).
type KeySequenceTimeoutMsg struct{}

func (KeySequenceTimeoutMsg) inputMessage() {}

// TrackSkipTimeoutMsg is sent after debounce delay for track skip operations.
// The Version field is used to ignore stale timeouts when rapid key presses occur.
type TrackSkipTimeoutMsg struct {
	Version int
}

func (TrackSkipTimeoutMsg) playbackMessage() {}

// LibraryScanProgressMsg wraps library scan progress updates.
type LibraryScanProgressMsg library.ScanProgress

func (LibraryScanProgressMsg) libraryScanMessage() {}

// LibraryScanCompleteMsg is sent when library scanning finishes.
type LibraryScanCompleteMsg struct {
	Stats *library.ScanStats
}

func (LibraryScanCompleteMsg) libraryScanMessage() {}

// TrackFinishedMsg is sent when the current track finishes playing.
type TrackFinishedMsg struct{}

func (TrackFinishedMsg) playbackMessage() {}

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
	// ViewDownloads shows the downloads monitor.
	ViewDownloads ViewMode = "downloads"
)

// SupportsDeepSearch returns true if the view mode supports deep search (g f).
func (v ViewMode) SupportsDeepSearch() bool {
	return v == ViewFileBrowser || v == ViewLibrary
}

// QueueAction represents the type of queue operation to perform.
type QueueAction int

const (
	// QueueAdd adds tracks to queue without interrupting current playback.
	QueueAdd QueueAction = iota
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

// LibraryDeleteContext stores context for library track deletion.
type LibraryDeleteContext struct {
	TrackID   int64
	TrackPath string
	Title     string
}

// FileDeleteContext stores context for file browser deletion.
type FileDeleteContext struct {
	Path  string
	Name  string
	IsDir bool
}

// InitStepMsg reports progress during async initialization.
type InitStepMsg struct {
	Step  string // Description of current step
	Error error  // Non-nil if initialization failed
	Done  bool   // True when initialization is complete
}

func (InitStepMsg) loadingMessage() {}

// LoadingTickMsg advances the loading animation.
type LoadingTickMsg struct{}

func (LoadingTickMsg) loadingMessage() {}

// ShowLoadingMsg is sent after the show delay to display the loading screen.
type ShowLoadingMsg struct{}

func (ShowLoadingMsg) loadingMessage() {}

// HideLoadingMsg is sent after minimum display time to hide the loading screen.
type HideLoadingMsg struct{}

func (HideLoadingMsg) loadingMessage() {}

// StderrMsg is sent when stderr output is captured from C libraries (ALSA, minimp3).
type StderrMsg struct {
	Line string
}

// InitResult holds the result of async initialization.
type InitResult struct {
	FileNav       any // navigator.Model[navigator.FileNode]
	LibNav        any // navigator.Model[library.Node]
	PlsNav        any // navigator.Model[playlists.Node]
	Queue         any // *playlist.PlayingQueue
	QueuePanel    any // queuepanel.Model
	SavedView     ViewMode
	IsFirstLaunch bool // True if no saved state exists
	Error         error
}

func (InitResult) loadingMessage() {}

// FavoriteMessage is implemented by messages related to favorites.
type FavoriteMessage interface {
	favoriteMessage()
}

// ToggleFavoriteMsg requests toggling favorite status for tracks.
type ToggleFavoriteMsg struct {
	TrackIDs []int64
}

func (ToggleFavoriteMsg) favoriteMessage() {}

// FavoritesChangedMsg is sent when favorites have been updated.
type FavoritesChangedMsg struct {
	Favorites map[int64]bool
}

func (FavoritesChangedMsg) favoriteMessage() {}

// QueueUndoMsg requests undoing the last queue operation.
type QueueUndoMsg struct{}

// QueueRedoMsg requests redoing the last undone queue operation.
type QueueRedoMsg struct{}

// DownloadMessage is implemented by messages related to downloads.
type DownloadMessage interface {
	tea.Msg
	downloadMessage()
}

// DownloadCreatedMsg is sent when a download is queued from the download popup.
type DownloadCreatedMsg struct {
	MBReleaseGroupID string
	MBReleaseID      string // Specific release selected for import
	MBArtistName     string
	MBAlbumTitle     string
	MBReleaseYear    string
	SlskdUsername    string
	SlskdDirectory   string
	Files            []DownloadFile
	// Full MusicBrainz data for importing
	MBReleaseGroup   *musicbrainz.ReleaseGroup   // Release group metadata
	MBReleaseDetails *musicbrainz.ReleaseDetails // Full release with tracks
}

func (DownloadCreatedMsg) downloadMessage() {}

// DownloadFile represents a file to download.
type DownloadFile struct {
	Filename string
	Size     int64
}

// DownloadsRefreshMsg is sent periodically to update download status from slskd.
type DownloadsRefreshMsg struct{}

func (DownloadsRefreshMsg) downloadMessage() {}

// DownloadsRefreshResultMsg contains the result of syncing with slskd.
type DownloadsRefreshResultMsg struct {
	Err error
}

func (DownloadsRefreshResultMsg) downloadMessage() {}

// DownloadDeletedMsg is sent after a download is deleted.
type DownloadDeletedMsg struct {
	ID  int64
	Err error
}

func (DownloadDeletedMsg) downloadMessage() {}

// CompletedDownloadsClearedMsg is sent after clearing completed downloads.
type CompletedDownloadsClearedMsg struct {
	Err error
}

func (CompletedDownloadsClearedMsg) downloadMessage() {}
