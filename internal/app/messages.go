// Package app contains application-level types and messages for the TUI.
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
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

// ServiceStateChangedMsg is sent when the playback service state changes.
type ServiceStateChangedMsg struct {
	Previous, Current int // playback.State values
}

func (ServiceStateChangedMsg) playbackMessage() {}

// ServiceTrackChangedMsg is sent when the current track changes.
type ServiceTrackChangedMsg struct {
	PreviousIndex int
	CurrentIndex  int
}

func (ServiceTrackChangedMsg) playbackMessage() {}

// ServiceClosedMsg is sent when the playback service is closed.
type ServiceClosedMsg struct{}

func (ServiceClosedMsg) playbackMessage() {}

// ServiceErrorMsg is sent when an error occurs in the playback service.
type ServiceErrorMsg struct {
	Operation string
	Path      string
	Err       error
}

func (ServiceErrorMsg) playbackMessage() {}

// ServiceQueueChangedMsg is sent when the queue contents change.
// Currently used to drain the subscription channel; may be used for future features.
type ServiceQueueChangedMsg struct{}

func (ServiceQueueChangedMsg) playbackMessage() {}

// AlbumArtUpdateMsg triggers album art preparation on the next tick.
// This is used to defer album art updates to ensure the playback service
// has fully updated its state after a queue change.
type AlbumArtUpdateMsg struct{}

func (AlbumArtUpdateMsg) playbackMessage() {}

// AlbumArtPreparedMsg is sent when async album art preparation completes.
type AlbumArtPreparedMsg struct {
	Path      string // Track path this was prepared for (for staleness check)
	ImageData []byte // Processed PNG data (nil or empty if no cover art)
}

func (AlbumArtPreparedMsg) playbackMessage() {}

// LyricsUpdateMsg triggers lyrics update when track changes.
// This is deferred to ensure track info (including duration) is available.
type LyricsUpdateMsg struct{}

func (LyricsUpdateMsg) playbackMessage() {}

// ServiceModeChangedMsg is sent when repeat/shuffle mode changes.
// Currently used to drain the subscription channel; may be used for future features.
type ServiceModeChangedMsg struct{}

func (ServiceModeChangedMsg) playbackMessage() {}

// ServicePositionChangedMsg is sent when a seek operation occurs.
// Currently used to drain the subscription channel; position updates come from TickMsg.
type ServicePositionChangedMsg struct{}

func (ServicePositionChangedMsg) playbackMessage() {}

// ViewMode represents the current navigator view type.
// Type alias for navctl.ViewMode.
type ViewMode = navctl.ViewMode

// QueueAction represents the type of queue operation to perform.
type QueueAction int

const (
	// QueueAdd adds tracks to queue without interrupting current playback.
	QueueAdd QueueAction = iota
	// QueueReplace clears the queue, adds tracks, and starts playing.
	QueueReplace
)

// InputMode represents the type of text input being collected.
// Type alias for popupctl.InputMode.
type InputMode = popupctl.InputMode

// InputMode constants for backward compatibility.
const (
	InputNone        = popupctl.InputNone
	InputNewPlaylist = popupctl.InputNewPlaylist
	InputNewFolder   = popupctl.InputNewFolder
	InputRename      = popupctl.InputRename
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
	Step string // Description of current step
	Err  error  // Non-nil if initialization failed
	Done bool   // True when initialization is complete
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
	FileNav                any // navigator.Model[navigator.FileNode]
	LibNav                 any // navigator.Model[library.Node]
	PlsNav                 any // navigator.Model[playlists.Node]
	Queue                  any // *playlist.PlayingQueue
	QueuePanel             any // queuepanel.Model
	SavedView              ViewMode
	SavedLibrarySubMode    string // "miller" or "album"
	SavedAlbumSelectedID   string // "artist:album" format
	SavedAlbumGroupFields  string // JSON: group field indices
	SavedAlbumSortCriteria string // JSON: sort criteria
	IsFirstLaunch          bool   // True if no saved state exists
	Err                    error
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

// RadioMessage is implemented by messages related to radio mode.
type RadioMessage interface {
	tea.Msg
	radioMessage()
}

// RadioFillResultMsg contains the result of filling the queue from radio.
type RadioFillResultMsg struct {
	Tracks []struct {
		ID          int64
		Path        string
		Title       string
		Artist      string
		Album       string
		TrackNumber int
	}
	Message string // Transient message (e.g., "No related tracks found")
	Err     error
}

func (RadioFillResultMsg) radioMessage() {}

// RadioToggledMsg is sent when radio mode is toggled.
type RadioToggledMsg struct {
	Enabled bool
}

func (RadioToggledMsg) radioMessage() {}

// Notification represents a temporary notification message.
type Notification struct {
	ID      int64
	Message string
}

// NotificationClearMsg is sent to clear a specific notification after a delay.
type NotificationClearMsg struct {
	ID int64
}

// NotificationDuration is how long notifications are displayed.
const NotificationDuration = 3 * time.Second

// NotificationClearCmd returns a command that clears the notification after a delay.
func NotificationClearCmd(id int64) tea.Cmd {
	return tea.Tick(NotificationDuration, func(time.Time) tea.Msg {
		return NotificationClearMsg{ID: id}
	})
}
