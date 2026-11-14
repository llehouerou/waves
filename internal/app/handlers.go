// internal/app/handlers.go
package app

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/handler"
	"github.com/llehouerou/waves/internal/keymap"
)

// closingMessage is what the screen says while the core closes.
const closingMessage = "Finishing the background work before quitting…"

// handleQuitKeys handles q and ctrl+c: the screen says waves is finishing its
// background work, and Update keeps it as it is until the core has closed.
func (m *Model) handleQuitKeys(key string) handler.Result {
	if m.Keys.Resolve(key) != keymap.ActionQuit {
		return handler.NotHandled
	}
	m.addNotification(closingMessage)
	m.closing = true
	return handler.Handled(closeCoreCmd(m.Core))
}

// addNotification shows message in the notification bar and returns its ID.
func (m *Model) addNotification(message string) int64 {
	m.nextNotificationID++
	m.Notifications = append(m.Notifications, Notification{ID: m.nextNotificationID, Message: message})
	m.ResizeComponents()
	return m.nextNotificationID
}

// closeCoreCmd closes the core, then ends the program. Close leaves the
// session bus, stops playback, waits for the background work, saves the
// queue and the volume a last time, and closes the database.
func closeCoreCmd(c io.Closer) tea.Cmd {
	return func() tea.Msg {
		// Run closes the core again once the program ended, gets the same
		// error and reports it.
		_ = c.Close() //nolint:errcheck // see above
		return tea.QuitMsg{}
	}
}
