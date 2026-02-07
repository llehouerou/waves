//go:build linux

package notify

import (
	"os"
	"testing"
)

func TestNewDBusNotifier(t *testing.T) {
	// Skip if no D-Bus session (CI environment)
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no D-Bus session available")
	}

	notifier, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if notifier == nil {
		t.Fatal("New() returned nil notifier")
	}
}

func TestNotifySendsNotification(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no D-Bus session available")
	}

	notifier, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	id, err := notifier.Notify(Notification{
		Title:   "Waves Test",
		Body:    "Test notification from unit test",
		Timeout: 1000, // 1 second
		Urgency: UrgencyLow,
	})
	if err != nil {
		t.Fatalf("Notify() error: %v", err)
	}
	// ID should be non-zero on success
	if id == 0 {
		t.Error("Notify() returned id=0, expected non-zero")
	}

	// Close it immediately
	if err := notifier.Close(id); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestNotifyReplacesExisting(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no D-Bus session available")
	}

	notifier, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Send first notification
	id1, err := notifier.Notify(Notification{
		Title:   "Track 1",
		Body:    "Artist - Album",
		Timeout: 2000,
	})
	if err != nil {
		t.Fatalf("first Notify() error: %v", err)
	}

	// Replace it
	id2, err := notifier.Notify(Notification{
		Title:      "Track 2",
		Body:       "Artist - Album",
		Timeout:    1000,
		ReplacesID: id1,
	})
	if err != nil {
		t.Fatalf("second Notify() error: %v", err)
	}

	// IDs should match when replacing
	if id2 != id1 {
		t.Errorf("replacing notification got id=%d, want id=%d", id2, id1)
	}

	if err := notifier.Close(id2); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}
