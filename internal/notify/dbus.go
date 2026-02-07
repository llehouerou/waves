//go:build linux

package notify

import (
	"github.com/godbus/dbus/v5"
)

const (
	dbusNotifyDest      = "org.freedesktop.Notifications"
	dbusNotifyPath      = "/org/freedesktop/Notifications"
	dbusNotifyInterface = "org.freedesktop.Notifications"
)

// dbusNotifier sends notifications via D-Bus.
type dbusNotifier struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// New creates a Notifier that sends desktop notifications via D-Bus.
// Returns a no-op notifier if D-Bus is unavailable.
func New() (Notifier, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		// D-Bus not available, return no-op notifier (intentional graceful degradation)
		return &stubNotifier{}, nil //nolint:nilerr // graceful fallback when D-Bus unavailable
	}

	obj := conn.Object(dbusNotifyDest, dbusNotifyPath)
	return &dbusNotifier{conn: conn, obj: obj}, nil
}

// Notify sends a notification via D-Bus.
func (n *dbusNotifier) Notify(notif Notification) (uint32, error) {
	// Build hints map
	hints := map[string]dbus.Variant{
		"urgency":       dbus.MakeVariant(byte(notif.Urgency)),
		"desktop-entry": dbus.MakeVariant("waves"),
	}

	// D-Bus Notify method signature:
	// Notify(app_name, replaces_id, icon, summary, body, actions, hints, timeout) -> id
	call := n.obj.Call(
		dbusNotifyInterface+".Notify",
		0,                // flags
		"Waves",          // app_name
		notif.ReplacesID, // replaces_id
		notif.Icon,       // app_icon (path or icon name)
		notif.Title,      // summary
		notif.Body,       // body
		[]string{},       // actions (empty for now)
		hints,            // hints
		notif.Timeout,    // expire_timeout
	)

	if call.Err != nil {
		return 0, call.Err
	}

	var id uint32
	if err := call.Store(&id); err != nil {
		return 0, err
	}

	return id, nil
}

// Close closes a notification by ID.
func (n *dbusNotifier) Close(id uint32) error {
	call := n.obj.Call(dbusNotifyInterface+".CloseNotification", 0, id)
	return call.Err
}

// stubNotifier is used when D-Bus is unavailable.
type stubNotifier struct{}

func (s *stubNotifier) Notify(_ Notification) (uint32, error) {
	return 0, nil
}

func (s *stubNotifier) Close(_ uint32) error {
	return nil
}
