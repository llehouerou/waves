package app

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/tags"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// The TUI learns what happens in the core from its event stream only.
func TestWatchServiceEvents_TurnsCoreEventsIntoMessages(t *testing.T) {
	m := newIntegrationTestModel()

	m.Core.Playback().AddTracks(core.Track{Path: "/a.mp3"})
	if msg := m.WatchServiceEvents()(); msg != (ServiceQueueChangedMsg{}) {
		t.Errorf("after a queue edit: %T, want ServiceQueueChangedMsg", msg)
	}

	m.Core.Playback().SetShuffle(true)
	if msg := m.WatchServiceEvents()(); msg != (ServiceModeChangedMsg{}) {
		t.Errorf("after a mode change: %T, want ServiceModeChangedMsg", msg)
	}

	if err := m.Core.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if msg := m.WatchServiceEvents()(); msg != (ServiceClosedMsg{}) {
		t.Errorf("after close: %T, want ServiceClosedMsg", msg)
	}
}

// Background work of the core that fails is shown, and the TUI keeps listening.
func TestUpdate_ServiceFailedMsg_ShowsTheErrorAndKeepsWatching(t *testing.T) {
	m := newIntegrationTestModel()

	m, cmd := updateModel(t, m, ServiceFailedMsg{Op: errmsg.OpRadioFill, Err: errors.New("last.fm is down")})

	if got := m.Popups.ErrorMsg(); !strings.Contains(got, string(errmsg.OpRadioFill)) || !strings.Contains(got, "last.fm is down") {
		t.Errorf("ErrorMsg = %q, want the radio fill failure", got)
	}
	if cmd == nil {
		t.Error("expected the command that waits for the next core event")
	}
}

// A volume set over MPRIS reaches the TUI as an event it did not cause: the
// player bar shows it, and the TUI keeps listening.
func TestUpdate_ServiceVolumeChangedMsg_RedrawsThePlayerBarAndKeepsWatching(t *testing.T) {
	m := newIntegrationTestModel()
	m.loadingState = loadingDone
	testPlayer(m.Core).SetState(player.Playing)
	testPlayer(m.Core).SetTrackInfo(&tags.FileInfo{Tag: tags.Tag{Title: "A"}})
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	for range 6 {
		m.Core.Playback().AdjustVolume(-0.10) // what MPRIS would set: a client gets no event for its own
	}

	m, cmd := updateModel(t, m, ServiceVolumeChangedMsg{})

	if view := testutil.StripANSI(m.View()); !strings.Contains(view, "40%") {
		t.Errorf("the player bar does not show a volume of 40%%:\n%s", view)
	}
	if cmd == nil {
		t.Fatal("expected the command that waits for the next core event")
	}
	m.Core.Playback().SetShuffle(true)
	if msg := cmd(); msg != (ServiceModeChangedMsg{}) {
		t.Errorf("next event: %T, want ServiceModeChangedMsg", msg)
	}
}

func TestHandlePlaybackKeys_Volume(t *testing.T) {
	m := newTestModel()
	pb := m.Core.Playback()

	m.handlePlaybackKeys("-")
	if got := pb.Volume(); got != 0.9 {
		t.Errorf("volume after - = %v, want 0.9", got)
	}

	m.handlePlaybackKeys("M")
	if !pb.Muted() {
		t.Error("not muted after M")
	}

	m.handlePlaybackKeys("+")
	if got := pb.Volume(); got != 1 || pb.Muted() {
		t.Errorf("after +: volume = %v muted = %v, want 1 unmuted", got, pb.Muted())
	}
}
