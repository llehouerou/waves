// internal/app/commands.go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

// WatchTrackFinished returns a command that waits for the player to finish.
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
