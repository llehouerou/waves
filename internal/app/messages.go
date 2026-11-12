// Package app contains application-level types and messages for the TUI.
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/app/popupctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
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
// Gen identifies the tick chain that produced it so stale chains are dropped
// (issue #28; see internal/app/tick.go and the TickMsg handler).
type TickMsg struct {
	Gen  int
	Time time.Time
}

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

// LibraryScanProgressMsg reports where the core's library scan is.
type LibraryScanProgressMsg core.ScanProgressed

func (LibraryScanProgressMsg) libraryScanMessage() {}

// LibraryScanCompleteMsg is sent when the core's library scan ends. Stats is
// nil when the scan failed.
type LibraryScanCompleteMsg struct {
	Stats *core.ScanStats
}

func (LibraryScanCompleteMsg) libraryScanMessage() {}

// ServiceStateChangedMsg is sent when the playback state changes.
type ServiceStateChangedMsg struct {
	Previous, Current int // core.State values
}

func (ServiceStateChangedMsg) playbackMessage() {}

// ServiceTrackChangedMsg is sent when the current track changes.
type ServiceTrackChangedMsg struct {
	PreviousIndex int
	CurrentIndex  int
}

func (ServiceTrackChangedMsg) playbackMessage() {}

// ServiceClosedMsg is sent when the core is closed.
type ServiceClosedMsg struct{}

func (ServiceClosedMsg) playbackMessage() {}

// ServiceErrorMsg is sent when a track could not be played.
type ServiceErrorMsg struct {
	Operation string
	Path      string
	Err       error
}

func (ServiceErrorMsg) playbackMessage() {}

// ServiceQueueChangedMsg is sent when the queue contents change.
type ServiceQueueChangedMsg struct{}

func (ServiceQueueChangedMsg) playbackMessage() {}

// AlbumArtUpdateMsg triggers album art preparation on the next tick.
// This is used to defer album art updates to ensure the playback service
// has fully updated its state after a queue change.
type AlbumArtUpdateMsg struct{}

func (AlbumArtUpdateMsg) playbackMessage() {}

// AlbumArtLoadedMsg carries album art read off the UI goroutine, ready to be
// handed to the terminal. PNG is nil when the track has no usable cover.
type AlbumArtLoadedMsg struct {
	Path string
	PNG  []byte
}

func (AlbumArtLoadedMsg) playbackMessage() {}

// NowPlayingNotifiedMsg carries the id of a notification sent off the UI
// goroutine, so the next one can replace it.
type NowPlayingNotifiedMsg struct {
	ID uint32
}

func (NowPlayingNotifiedMsg) playbackMessage() {}

// LyricsUpdateMsg triggers lyrics update when track changes.
// This is deferred to ensure track info (including duration) is available.
type LyricsUpdateMsg struct{}

func (LyricsUpdateMsg) playbackMessage() {}

// ServiceModeChangedMsg is sent when repeat/shuffle mode changes.
type ServiceModeChangedMsg struct{}

func (ServiceModeChangedMsg) playbackMessage() {}

// ServiceFailedMsg is sent when background work of the core failed: a radio
// fill, saving the queue or the volume, a Sync of the downloads.
type ServiceFailedMsg struct {
	Op  errmsg.Op
	Err error
}

func (ServiceFailedMsg) playbackMessage() {}

// ServicePositionChangedMsg is sent when a seek operation occurs.
// The TUI ignores it: position updates come from TickMsg.
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
	LibNav                 any // navigator.Model[librarynav.Node]
	PlsNav                 any // navigator.Model[playlistnav.Node]
	LibraryBrowser         any // librarybrowser.Model
	SavedView              ViewMode
	SavedLibrarySubMode    string // "miller", "album", or "browser"
	SavedAlbumSelectedID   string // "artist:album" format
	SavedAlbumGroupFields  string // JSON: group field indices
	SavedAlbumSortCriteria string // JSON: sort criteria
	SavedBrowserState      string // "artist\x00album\x00trackID" format
	IsFirstLaunch          bool   // True if no saved state exists
	Err                    error
}

func (InitResult) loadingMessage() {}

// QueueUndoMsg requests undoing the last queue operation.
type QueueUndoMsg struct{}

// QueueRedoMsg requests redoing the last undone queue operation.
type QueueRedoMsg struct{}

// DownloadMessage is implemented by messages related to downloads.
type DownloadMessage interface {
	tea.Msg
	downloadMessage()
}

// ServiceDownloadsChangedMsg is sent when the core's Sync, or a command since
// the last one, left the downloads different from what the core last said.
type ServiceDownloadsChangedMsg struct {
	Downloads []core.Download
}

func (ServiceDownloadsChangedMsg) downloadMessage() {}

// ServiceSearchMsg carries an event of the slskd search the core runs, a
// core.SearchProgressed or a core.SearchFinished, for the download popup.
type ServiceSearchMsg struct {
	Event core.Event
}

func (ServiceSearchMsg) downloadMessage() {}

// ServiceImportMsg carries an event of the import the core runs, a
// core.ImportProgressed or a core.ImportFinished, for the import popup when
// it is still open, and for the views the import changes.
type ServiceImportMsg struct {
	Event core.Event
}

func (ServiceImportMsg) downloadMessage() {}

// DownloadsChangedMsg carries the result of one download operation and the
// list as it stands after it. Downloads is nil only when the list couldn't be
// read; an empty list is non-nil.
type DownloadsChangedMsg struct {
	Op        errmsg.Op
	Downloads []core.Download
	Err       error
}

func (DownloadsChangedMsg) downloadMessage() {}

// ReleasesRefreshedMsg reports the end of the ListenBrainz refresh.
type ReleasesRefreshedMsg struct {
	Err error
}

// SimilarWarmupMsg ticks the similar-artists warm-up; only its end is acted on.
type SimilarWarmupMsg struct {
	Done bool
}

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
