// internal/app/commands.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/downloads"
	importpopup "github.com/llehouerou/waves/internal/importer/popup"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/stderr"
)

// TickCmd returns a command that sends a TickMsg tagged with gen after 1s.
func TickCmd(gen int) tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg{Gen: gen, Time: t}
	})
}

// KeySequenceTimeoutCmd returns a command that sends KeySequenceTimeoutMsg after 300ms.
func KeySequenceTimeoutCmd() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(_ time.Time) tea.Msg {
		return KeySequenceTimeoutMsg{}
	})
}

// TrackSkipTimeoutCmd returns a command that sends TrackSkipTimeoutMsg after 350ms.
func TrackSkipTimeoutCmd(version int) tea.Cmd {
	return tea.Tick(350*time.Millisecond, func(_ time.Time) tea.Msg {
		return TrackSkipTimeoutMsg{Version: version}
	})
}

// WatchServiceEvents returns a command that waits for playback service events.
// It listens on all subscription channels and converts events to tea.Msg.
func (m Model) WatchServiceEvents() tea.Cmd {
	if m.playbackSub == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case e := <-m.playbackSub.StateChanged:
			return ServiceStateChangedMsg{
				Previous: int(e.Previous),
				Current:  int(e.Current),
			}
		case e := <-m.playbackSub.TrackChanged:
			return ServiceTrackChangedMsg{
				PreviousIndex: e.PreviousIndex,
				CurrentIndex:  e.Index,
			}
		case <-m.playbackSub.QueueChanged:
			return ServiceQueueChangedMsg{}
		case <-m.playbackSub.ModeChanged:
			return ServiceModeChangedMsg{}
		case <-m.playbackSub.PositionChanged:
			// Drain position events; position updates come from TickMsg
			return ServicePositionChangedMsg{}
		case e := <-m.playbackSub.Error:
			return ServiceErrorMsg{
				Operation: e.Operation,
				Path:      e.Path,
				Err:       e.Err,
			}
		case <-m.playbackSub.Done:
			return ServiceClosedMsg{}
		}
	}
}

// LoadingTickCmd returns a command that sends LoadingTickMsg for animation.
func LoadingTickCmd() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(_ time.Time) tea.Msg {
		return LoadingTickMsg{}
	})
}

// ShowLoadingAfterDelayCmd returns a command that sends ShowLoadingMsg after 400ms.
// This delays showing the loading screen so fast loads don't flash.
func ShowLoadingAfterDelayCmd() tea.Cmd {
	return tea.Tick(400*time.Millisecond, func(_ time.Time) tea.Msg {
		return ShowLoadingMsg{}
	})
}

// HideLoadingAfterMinTimeCmd returns a command that sends HideLoadingMsg after 800ms.
// This ensures the loading screen stays visible long enough to be understood.
func HideLoadingAfterMinTimeCmd() tea.Cmd {
	return tea.Tick(800*time.Millisecond, func(_ time.Time) tea.Msg {
		return HideLoadingMsg{}
	})
}

// HideLoadingFirstLaunchCmd returns a command that sends HideLoadingMsg after 3 seconds.
// Used on first launch to display the loading screen longer.
func HideLoadingFirstLaunchCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
		return HideLoadingMsg{}
	})
}

// waitForChannel creates a command that waits for a value from a channel and converts it to a message.
// onResult receives the value and a boolean indicating if the channel is still open (false means channel closed).
func waitForChannel[T any](ch <-chan T, onResult func(T, bool) tea.Msg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		result, ok := <-ch
		return onResult(result, ok)
	}
}

// WatchStderr returns a command that waits for stderr output from C libraries.
func WatchStderr() tea.Cmd {
	return func() tea.Msg {
		line, ok := <-stderr.Messages
		if !ok {
			return nil // Channel closed
		}
		return StderrMsg{Line: line}
	}
}

// DownloadsRefreshTickCmd returns a command that sends DownloadsRefreshMsg after 3 seconds.
func DownloadsRefreshTickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
		return DownloadsRefreshMsg{}
	})
}

// RefreshDownloadsCmd syncs the downloads with slskd.
func RefreshDownloadsCmd(dlMgr *downloads.Manager) tea.Cmd {
	return func() tea.Msg {
		return DownloadsRefreshResultMsg{Err: dlMgr.Sync()}
	}
}

// CreateDownloadCmd persists a new download to the database.
func CreateDownloadCmd(dlMgr *downloads.Manager, msg DownloadCreatedMsg) tea.Cmd {
	return func() tea.Msg {
		dl := downloads.Download{
			MBReleaseGroupID: msg.MBReleaseGroupID,
			MBReleaseID:      msg.MBReleaseID,
			MBArtistName:     msg.MBArtistName,
			MBAlbumTitle:     msg.MBAlbumTitle,
			MBReleaseYear:    msg.MBReleaseYear,
			MBReleaseGroup:   msg.MBReleaseGroup,
			MBReleaseDetails: msg.MBReleaseDetails,
			SlskdUsername:    msg.SlskdUsername,
			SlskdDirectory:   msg.SlskdDirectory,
		}

		// Convert files
		for _, f := range msg.Files {
			dl.Files = append(dl.Files, downloads.DownloadFile{
				Filename: f.Filename,
				Size:     f.Size,
			})
		}

		_, err := dlMgr.Create(dl)
		if err != nil {
			return DownloadsRefreshResultMsg{Err: err}
		}

		// Return success (will trigger refresh)
		return DownloadsRefreshResultMsg{}
	}
}

// DeleteDownloadCmd deletes a download: its slskd transfers, files and row.
func DeleteDownloadCmd(dlMgr *downloads.Manager, id int64) tea.Cmd {
	return func() tea.Msg {
		return DownloadDeletedMsg{ID: id, Err: dlMgr.Delete(id)}
	}
}

// RetryFailedDownloadCmd re-queues the failed files of a download on slskd,
// then syncs so the view shows them queued again.
func RetryFailedDownloadCmd(dlMgr *downloads.Manager, id int64) tea.Cmd {
	return func() tea.Msg {
		if err := dlMgr.Retry(id); err != nil {
			return DownloadRetriedMsg{Err: err}
		}
		return DownloadsRefreshResultMsg{Err: dlMgr.Sync()}
	}
}

// ClearCompletedDownloadsCmd forgets all completed downloads.
func ClearCompletedDownloadsCmd(dlMgr *downloads.Manager) tea.Cmd {
	return func() tea.Msg {
		return CompletedDownloadsClearedMsg{Err: dlMgr.ClearCompleted()}
	}
}

// AddTracksToLibraryParams contains parameters for adding tracks to the library.
type AddTracksToLibraryParams struct {
	Library      *library.Library
	Paths        []string
	DownloadID   int64
	ArtistName   string
	AlbumName    string
	AllSucceeded bool
}

// AddTracksToLibraryCmd adds specific tracks to the library without a full refresh.
func AddTracksToLibraryCmd(params AddTracksToLibraryParams) tea.Cmd {
	return func() tea.Msg {
		err := params.Library.AddTracks(params.Paths)
		return importpopup.LibraryRefreshedMsg{
			Err:          err,
			DownloadID:   params.DownloadID,
			ArtistName:   params.ArtistName,
			AlbumName:    params.AlbumName,
			AllSucceeded: params.AllSucceeded,
		}
	}
}
