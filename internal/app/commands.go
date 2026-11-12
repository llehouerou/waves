// internal/app/commands.go
package app

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
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

// WatchServiceEvents returns a command that waits for the next event of the
// core and turns it into a tea.Msg.
func (m Model) WatchServiceEvents() tea.Cmd {
	if m.Core == nil {
		return nil
	}
	events := m.Core.Events()
	return func() tea.Msg {
		switch e := (<-events).(type) {
		case core.StateChanged:
			return ServiceStateChangedMsg{
				Previous: int(e.Previous),
				Current:  int(e.Current),
			}
		case core.TrackChanged:
			return ServiceTrackChangedMsg{
				PreviousIndex: e.PreviousIndex,
				CurrentIndex:  e.Index,
			}
		case core.QueueChanged:
			return ServiceQueueChangedMsg{}
		case core.ModeChanged:
			return ServiceModeChangedMsg{}
		case core.PositionChanged:
			return ServicePositionChangedMsg{}
		case core.PlaybackFailed:
			return ServiceErrorMsg{
				Operation: e.Operation,
				Path:      e.Path,
				Err:       e.Err,
			}
		case core.ScanProgressed:
			return LibraryScanProgressMsg(e)
		case core.ScanFinished:
			return LibraryScanCompleteMsg{Stats: e.Stats}
		case core.DownloadsChanged:
			return ServiceDownloadsChangedMsg(e)
		case core.SearchProgressed, core.SearchFinished:
			return ServiceSearchMsg{Event: e}
		case core.ImportProgressed, core.ImportFinished:
			return ServiceImportMsg{Event: e}
		case core.RetagProgressed, core.RetagFinished:
			return ServiceRetagMsg{Event: e}
		case core.ReleasesRefreshed, core.WarmupFinished:
			return ServiceReleasesMsg{Event: e}
		case core.ExportProgressed, core.ExportFinished:
			return ServiceExportMsg{Event: e}
		case core.Failed:
			return ServiceFailedMsg{Op: e.Op, Err: e.Err}
		default:
			// nil: the core closed its event stream.
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

// downloadsCmd runs one download operation off the UI goroutine (op may be
// nil to only re-read the list) and reports it with the list that follows:
// a command of the core returns its error and emits nothing.
func downloadsCmd(dls *core.Downloads, name errmsg.Op, op func(*core.Downloads) error) tea.Cmd {
	return func() tea.Msg {
		var err error
		if op != nil {
			err = op(dls)
		}
		list, listErr := dls.List()
		return DownloadsChangedMsg{Op: name, Downloads: list, Err: errors.Join(err, listErr)}
	}
}

// loadDownloadsCmd re-reads the downloads list.
func loadDownloadsCmd(dls *core.Downloads) tea.Cmd {
	return downloadsCmd(dls, errmsg.OpDownloadRefresh, nil)
}
