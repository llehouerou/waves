package player

import (
	"errors"
	"testing"
)

// Like the real player, a play that fails to open has already stopped what was
// playing, and starts no track.
func TestMock_FailedPlayStopsAndStartsNothing(t *testing.T) {
	m := NewMock()
	if err := m.Play("/music/a.mp3"); err != nil {
		t.Fatal(err)
	}

	m.SetPlayError(errors.New("unreadable"))
	if err := m.Play("/music/b.mp3"); err == nil {
		t.Fatal("Play succeeded, want the open failure")
	}

	if got := m.State(); got != Stopped {
		t.Errorf("State() = %v, want Stopped", got)
	}
	if got := m.TrackStarts(); got != 1 {
		t.Errorf("TrackStarts() = %d, want 1", got)
	}
}
