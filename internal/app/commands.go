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
