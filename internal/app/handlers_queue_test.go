package app

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/albumview"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

func queuePaths(m Model) []string {
	tracks := m.Core.Playback().QueueTracks()
	paths := make([]string, 0, len(tracks))
	for _, track := range tracks {
		paths = append(paths, track.Path)
	}
	return paths
}

func assertQueue(t *testing.T, m Model, want ...string) {
	t.Helper()
	if got := queuePaths(m); !slices.Equal(got, want) {
		t.Errorf("queue = %v, want %v", got, want)
	}
}

func TestQueueHistoryKeys_CtrlZUndoesAndCtrlYRedoes(t *testing.T) {
	m := newIntegrationTestModel(t)
	m.Core.Playback().AddTracks(core.Track{Path: "/a"}, core.Track{Path: "/b"}, core.Track{Path: "/c"})
	m.Core.Playback().RemoveTracks([]int{0})
	assertQueue(t, m, "/b", "/c")

	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlZ})
	assertQueue(t, m, "/a", "/b", "/c")

	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlY})
	assertQueue(t, m, "/b", "/c")
}

func TestQueueHistoryKeys_CtrlYWithNothingToRedoChangesNothing(t *testing.T) {
	m := newIntegrationTestModel(t)
	m.Core.Playback().AddTracks(core.Track{Path: "/a"}, core.Track{Path: "/b"})

	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlY})

	assertQueue(t, m, "/a", "/b")
}

// Undo and redo keep the queue on the track playing: the queue panel shows it
// playing, at the place the restored queue puts it.
func TestQueueHistoryKeys_KeepTheTrackPlayingCurrent(t *testing.T) {
	m := newIntegrationTestModel(t)
	m.Layout.ShowQueue()
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	pb := m.Core.Playback()
	pb.AddTracks(
		core.Track{Path: "/a", Title: "Alpha"},
		core.Track{Path: "/b", Title: "Bravo"},
		core.Track{Path: "/c", Title: "Charlie"},
	)
	pb.QueueMoveTo(2)
	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	pb.RemoveTracks([]int{0})

	expectPlaying := func(header string) {
		t.Helper()
		view := testutil.StripANSI(m.Layout.QueuePanel().View())
		if !strings.Contains(view, header) {
			t.Errorf("no header %q; view:\n%s", header, view)
		}
		if line := testutil.FindLine(view, "Charlie"); !strings.Contains(line, "▶") {
			t.Errorf("Charlie's line %q, want it playing; view:\n%s", line, view)
		}
		for _, other := range []string{"Alpha", "Bravo"} {
			if line := testutil.FindLine(view, other); strings.Contains(line, "▶") {
				t.Errorf("%s's line %q shows it playing; view:\n%s", other, line, view)
			}
		}
	}

	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlZ})
	expectPlaying("Queue (3/3)")

	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyCtrlY})
	expectPlaying("Queue (2/2)")
}

// Queueing an album from the album view, with Enter or the add key, shows a
// failure to read its tracks as the other queueing paths do; an album with no
// track queues nothing and shows nothing.
func TestAlbumViewQueueAlbum_ShowsAFailureToReadItsTracks(t *testing.T) {
	for _, replace := range []bool{true, false} {
		queue := func(m Model) Model {
			m, _ = updateModel(t, m, action.Msg{Source: "albumview", Action: albumview.QueueAlbum{
				AlbumArtist: "Album Artist", Album: "Album Title", Replace: replace,
			}})
			return m
		}

		m := newIntegrationTestModel(t)
		if m = queue(m); m.Popups.ErrorMsg() != "" || len(queuePaths(m)) != 0 {
			t.Errorf("replace %v, album with no track: error %q, queue %v; want none, empty", replace, m.Popups.ErrorMsg(), queuePaths(m))
		}

		if err := m.Core.Close(); err != nil { // the library can no longer be read
			t.Fatalf("close core: %v", err)
		}
		if msg := queue(m).Popups.ErrorMsg(); !strings.Contains(msg, string(errmsg.OpQueueAdd)) {
			t.Errorf("replace %v, tracks unreadable: error %q, want an %q error", replace, msg, errmsg.OpQueueAdd)
		}
	}
}
