// internal/app/commands.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/stderr"
)

// TickCmd returns a command that sends TickMsg after 1 second.
func TickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
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

// WatchTrackFinished returns a command that waits for the player to finish naturally.
// Returns TrackFinishedMsg only for natural track completion, not manual stops.
func (m Model) WatchTrackFinished() tea.Cmd {
	return func() tea.Msg {
		<-m.Playback.FinishedChan()
		return TrackFinishedMsg{}
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

// DownloadsRefreshTickCmd returns a command that sends DownloadsRefreshMsg after 30 seconds.
func DownloadsRefreshTickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(_ time.Time) tea.Msg {
		return DownloadsRefreshMsg{}
	})
}

// RefreshDownloadsCmd fetches download status from slskd and syncs with local database.
// It also verifies completed files on disk if completedPath is set.
func RefreshDownloadsCmd(dlMgr *downloads.Manager, client *slskd.Client, completedPath string) tea.Cmd {
	return func() tea.Msg {
		slskdDownloads, err := client.GetDownloads()
		if err != nil {
			return DownloadsRefreshResultMsg{Err: err}
		}

		if err := dlMgr.UpdateFromSlskd(slskdDownloads); err != nil {
			return DownloadsRefreshResultMsg{Err: err}
		}

		// Verify completed files on disk
		if completedPath != "" {
			if err := dlMgr.VerifyOnDisk(completedPath); err != nil {
				return DownloadsRefreshResultMsg{Err: err}
			}
		}

		return DownloadsRefreshResultMsg{}
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

// DeleteDownloadCmd removes a download from the database.
func DeleteDownloadCmd(dlMgr *downloads.Manager, id int64) tea.Cmd {
	return func() tea.Msg {
		err := dlMgr.Delete(id)
		return DownloadDeletedMsg{ID: id, Err: err}
	}
}

// ClearCompletedDownloadsCmd removes all completed downloads.
func ClearCompletedDownloadsCmd(dlMgr *downloads.Manager) tea.Cmd {
	return func() tea.Msg {
		err := dlMgr.DeleteCompleted()
		return CompletedDownloadsClearedMsg{Err: err}
	}
}
