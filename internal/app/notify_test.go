package app

import (
	"testing"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/notify"
	"github.com/llehouerou/waves/internal/playback"
)

// mockNotifier records notifications for testing.
type mockNotifier struct {
	notifications []notify.Notification
	lastID        uint32
}

func (m *mockNotifier) Notify(n notify.Notification) (uint32, error) {
	m.lastID++
	m.notifications = append(m.notifications, n)
	return m.lastID, nil
}

func (m *mockNotifier) Close(_ uint32) error {
	return nil
}

func TestSendNowPlayingNotification(t *testing.T) {
	mock := &mockNotifier{}
	enabled := true
	cfg := config.NotificationsConfig{
		Enabled:      &enabled,
		NowPlaying:   &enabled,
		ShowAlbumArt: &enabled,
		Timeout:      5000,
	}

	track := &playback.Track{
		Path:   "/music/artist/album/song.mp3",
		Title:  "Test Song",
		Artist: "Test Artist",
		Album:  "Test Album",
	}

	m := &Model{
		notifier:            mock,
		notificationsConfig: cfg,
	}

	m.sendNowPlayingNotification(track)

	if len(mock.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(mock.notifications))
	}

	n := mock.notifications[0]
	if n.Title != "Test Song" {
		t.Errorf("Title = %q, want %q", n.Title, "Test Song")
	}
	if n.Body != "Test Artist · Test Album" {
		t.Errorf("Body = %q, want %q", n.Body, "Test Artist · Test Album")
	}
	if n.Urgency != notify.UrgencyLow {
		t.Errorf("Urgency = %d, want UrgencyLow", n.Urgency)
	}
}

func TestSendNowPlayingNotificationDisabled(t *testing.T) {
	mock := &mockNotifier{}
	disabled := false
	cfg := config.NotificationsConfig{
		Enabled:    &disabled,
		NowPlaying: &disabled,
	}

	track := &playback.Track{
		Title:  "Test",
		Artist: "Test",
	}

	m := &Model{
		notifier:            mock,
		notificationsConfig: cfg,
	}

	m.sendNowPlayingNotification(track)

	if len(mock.notifications) != 0 {
		t.Errorf("expected 0 notifications when disabled, got %d", len(mock.notifications))
	}
}

func TestSendNowPlayingNotificationNilNotifier(_ *testing.T) {
	m := &Model{
		notifier: nil, // No notifier
	}

	// Should not panic
	m.sendNowPlayingNotification(&playback.Track{})
}

func TestSendNowPlayingNotificationReplacesID(t *testing.T) {
	mock := &mockNotifier{}
	enabled := true
	cfg := config.NotificationsConfig{
		Enabled:    &enabled,
		NowPlaying: &enabled,
		Timeout:    5000,
	}

	m := &Model{
		notifier:            mock,
		notificationsConfig: cfg,
		lastNowPlayingID:    42, // Previous notification ID
	}

	track := &playback.Track{Title: "Song", Artist: "Artist", Album: "Album"}
	m.sendNowPlayingNotification(track)

	if len(mock.notifications) != 1 {
		t.Fatal("expected 1 notification")
	}
	if mock.notifications[0].ReplacesID != 42 {
		t.Errorf("ReplacesID = %d, want 42", mock.notifications[0].ReplacesID)
	}
	if m.lastNowPlayingID != 1 {
		t.Errorf("lastNowPlayingID = %d, want 1", m.lastNowPlayingID)
	}
}

func TestSendDownloadCompleteNotification(t *testing.T) {
	mock := &mockNotifier{}
	enabled := true
	cfg := config.NotificationsConfig{
		Enabled:   &enabled,
		Downloads: &enabled,
		Timeout:   5000,
	}

	m := &Model{
		notifier:            mock,
		notificationsConfig: cfg,
	}

	m.sendDownloadCompleteNotification("Test Artist", "Test Album")

	if len(mock.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(mock.notifications))
	}

	n := mock.notifications[0]
	if n.Title != "Download Complete" {
		t.Errorf("Title = %q, want %q", n.Title, "Download Complete")
	}
	if n.Body != "Test Artist - Test Album" {
		t.Errorf("Body = %q, want %q", n.Body, "Test Artist - Test Album")
	}
	if n.Urgency != notify.UrgencyNormal {
		t.Errorf("Urgency = %d, want UrgencyNormal", n.Urgency)
	}
}

func TestSendDownloadCompleteNotificationDisabled(t *testing.T) {
	mock := &mockNotifier{}
	disabled := false
	cfg := config.NotificationsConfig{
		Enabled:   &disabled,
		Downloads: &disabled,
	}

	m := &Model{
		notifier:            mock,
		notificationsConfig: cfg,
	}

	m.sendDownloadCompleteNotification("Artist", "Album")

	if len(mock.notifications) != 0 {
		t.Errorf("expected 0 notifications when disabled, got %d", len(mock.notifications))
	}
}
