package app

import (
	"database/sql"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/app/navctl"
	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/action"
	"github.com/llehouerou/waves/internal/ui/albumview"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

var (
	alpha   = core.Track{ID: 1, Path: "/a.mp3", Title: "Alpha"}
	bravo   = core.Track{ID: 2, Path: "/b.mp3", Title: "Bravo"}
	charlie = core.Track{ID: 3, Path: "/c.mp3", Title: "Charlie"}
)

// newSelectionTestModel returns a model with tracks queued, nothing playing,
// the queue panel shown and focused.
func newSelectionTestModel(t *testing.T, tracks ...core.Track) Model {
	t.Helper()
	m := newIntegrationTestModel(t)
	m.Core.Playback().AddTracks(tracks...)
	return showQueuePanel(t, m)
}

func showQueuePanel(t *testing.T, m Model) Model {
	t.Helper()
	m.Layout.ShowQueue()
	m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.SetFocus(navctl.FocusQueue)
	return m
}

// queueKeyMsg builds the key messages the selection tests press.
func queueKeyMsg(key string) tea.Msg {
	switch key {
	case "ctrl+z":
		return tea.KeyMsg{Type: tea.KeyCtrlZ}
	case "ctrl+y":
		return tea.KeyMsg{Type: tea.KeyCtrlY}
	case "ctrl+a":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	}
	return keyMsg(key)
}

// pressWithoutEvent sends the keys to the model with the action messages of
// their commands, and no queue-changed message.
func pressWithoutEvent(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, key := range keys {
		var cmd tea.Cmd
		m, cmd = updateModel(t, m, queueKeyMsg(key))
		msgs, _ := drain(t, cmd)
		for _, msg := range msgs {
			if _, ok := msg.(action.Msg); ok {
				m, _ = updateModel(t, m, msg)
			}
		}
	}
	return m
}

// pressEach sends the keys to the model, each followed by the action message of
// its command and by the queue-changed message the core's event becomes.
func pressEach(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, key := range keys {
		m = pressWithoutEvent(t, m, key)
		m, _ = updateModel(t, m, ServiceQueueChangedMsg{})
	}
	return m
}

// emitted returns the action of the queue panel a key emits, unhandled.
func emitted(t *testing.T, m Model, key string) action.Action {
	t.Helper()
	_, cmd := updateModel(t, m, queueKeyMsg(key))
	msgs, _ := drain(t, cmd)
	for _, msg := range msgs {
		if a, ok := msg.(action.Msg); ok {
			return a.Action
		}
	}
	t.Fatalf("%s emitted no action", key)
	return nil
}

// replaceQueue replaces the queue behind the panel's back, then delivers the
// queue-changed message: the panel learns of the replace from the event only.
func replaceQueue(t *testing.T, m Model, tracks ...core.Track) Model {
	t.Helper()
	m.Core.Playback().ReplaceTracks(tracks...)
	m.Layout.QueuePanel().SyncCursor()
	m, _ = updateModel(t, m, ServiceQueueChangedMsg{})
	return m
}

// panelRows returns the title of each row the queue panel shows, followed by
// " ●" on a marked row.
func panelRows(m Model) []string {
	tracks := m.Core.Playback().QueueTracks()
	view := testutil.StripANSI(m.Layout.QueuePanel().View())
	var rows []string
	for line := range strings.SplitSeq(view, "\n") {
		if len(rows) == len(tracks) || strings.Contains(line, "Queue") {
			continue
		}
		if title := tracks[len(rows)].Title; strings.Contains(line, title) {
			if strings.Contains(line, "●") {
				title += " ●"
			}
			rows = append(rows, title)
		}
	}
	return rows
}

func assertRows(t *testing.T, m Model, want ...string) {
	t.Helper()
	if got := panelRows(m); !slices.Equal(got, want) {
		t.Errorf("queue panel rows = %q, want %q", got, want)
	}
}

func assertHeader(t *testing.T, m Model, want string) {
	t.Helper()
	view := testutil.StripANSI(m.Layout.QueuePanel().View())
	if !strings.Contains(view, want) {
		t.Errorf("no %q in the header; view:\n%s", want, view)
	}
}

func assertNoSelection(t *testing.T, m Model) {
	t.Helper()
	if view := testutil.StripANSI(m.Layout.QueuePanel().View()); strings.Contains(view, "selected") {
		t.Errorf("the header counts a selection; view:\n%s", view)
	}
	if m.Layout.QueuePanel().HasSelection() {
		t.Error("the panel has a selection")
	}
}

func TestQueueSelection_UndoKeepsTheMarkOnItsTrack(t *testing.T) {
	m := newSelectionTestModel(t, alpha, bravo, charlie)

	m = pressEach(t, m, "d", "x", "ctrl+z")

	assertQueue(t, m, "/a.mp3", "/b.mp3", "/c.mp3")
	assertRows(t, m, "Alpha", "Bravo ●", "Charlie")
	assertHeader(t, m, "[1 selected]")

	m = pressEach(t, m, "d")
	assertQueue(t, m, "/a.mp3", "/c.mp3")
}

func TestQueueSelection_RedoKeepsTheMarkOnItsTrack(t *testing.T) {
	m := newSelectionTestModel(t, alpha, bravo, charlie)

	m = pressEach(t, m, "d", "ctrl+z", "j", "x", "ctrl+y")

	assertQueue(t, m, "/b.mp3", "/c.mp3")
	assertRows(t, m, "Bravo ●", "Charlie")

	m = pressEach(t, m, "d")
	assertQueue(t, m, "/c.mp3")
}

func TestQueueSelection_UndoOfAMoveKeepsTheMarkOnTheMovedTrack(t *testing.T) {
	m := newSelectionTestModel(t, alpha, bravo, charlie)

	m = pressEach(t, m, "x", "J")
	assertRows(t, m, "Bravo", "Alpha ●", "Charlie")
	m = pressEach(t, m, "ctrl+z")

	assertRows(t, m, "Alpha ●", "Bravo", "Charlie")
	if got, want := emitted(t, m, "F"), (queuepanel.ToggleFavorite{TrackIDs: []int64{alpha.ID}}); !reflect.DeepEqual(got, want) {
		t.Errorf("F emitted %#v, want %#v", got, want)
	}
	if got, want := emitted(t, m, "ctrl+a"), (queuepanel.AddToPlaylist{TrackIDs: []int64{alpha.ID}}); !reflect.DeepEqual(got, want) {
		t.Errorf("ctrl+a emitted %#v, want %#v", got, want)
	}

	m = pressEach(t, m, "ctrl+y")
	assertRows(t, m, "Bravo", "Alpha ●", "Charlie")

	m = pressEach(t, m, "ctrl+z", "D")
	assertQueue(t, m, "/a.mp3")
}

func TestQueueSelection_AReplacedQueueHasNoMark(t *testing.T) {
	for _, tt := range []struct {
		name   string
		tracks []core.Track
	}{
		{"other tracks", []core.Track{{Path: "/x.mp3", Title: "Xray"}, {Path: "/y.mp3", Title: "Yankee"}, {Path: "/z.mp3", Title: "Zulu"}}},
		{"the same files", []core.Track{alpha, bravo, charlie}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newSelectionTestModel(t, alpha, bravo, charlie)
			m = pressEach(t, m, "j", "x")

			m = replaceQueue(t, m, tt.tracks...)

			assertRows(t, m, tt.tracks[0].Title, tt.tracks[1].Title, tt.tracks[2].Title)
			assertNoSelection(t, m)
			// The replace put the queue position, and the cursor, on the first.
			m = pressEach(t, m, "d")
			assertQueue(t, m, tt.tracks[1].Path, tt.tracks[2].Path)
		})
	}
}

func TestQueueSelection_UndoOfAReplaceBringsNoMarkBack(t *testing.T) {
	m := newSelectionTestModel(t, alpha, bravo, charlie)
	m = pressEach(t, m, "j", "x")
	m = replaceQueue(t, m, core.Track{Path: "/x.mp3", Title: "Xray"})

	m = pressEach(t, m, "ctrl+z")

	assertRows(t, m, "Alpha", "Bravo", "Charlie")
	assertNoSelection(t, m)
}

// Each replace of the app prunes the selection: an undo pressed before the
// queue-changed message brings no mark back.
func TestQueueSelection_UndoOfAReplaceBeforeTheEventBringsNoMarkBack(t *testing.T) {
	for _, tt := range []struct {
		name    string
		replace func(t *testing.T, m Model) Model
	}{
		{"queue action", func(_ *testing.T, m Model) Model {
			m.HandleQueueAction(QueueReplace)
			return m
		}},
		{"container and play", func(_ *testing.T, m Model) Model {
			m.HandleContainerAndPlay()
			return m
		}},
		{"album view", func(t *testing.T, m Model) Model {
			m, _ = updateModel(t, m, action.Msg{Source: "albumview", Action: albumview.QueueAlbum{
				AlbumArtist: "Album Artist", Album: "Album Title", Replace: true,
			}})
			return m
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newLibraryTestModel(t, func(db *sql.DB) {
				insertSingleAlbum(t, db, "Album Artist", "Album Title", 3)
			})
			m.Core.Playback().AddTracks(alpha, bravo, charlie)
			m = showQueuePanel(t, m)
			m = pressEach(t, m, "j", "x")

			m = tt.replace(t, m)
			if got := queuePaths(m); len(got) != 3 || got[0] != "/lib/Album Title/1.mp3" {
				t.Fatalf("queue after the replace = %v, want the album", got)
			}
			m = pressWithoutEvent(t, m, "ctrl+z")

			assertRows(t, m, "Alpha", "Bravo", "Charlie")
			assertNoSelection(t, m)
		})
	}
}

// The panel's own removal prunes the selection too: a track marked between
// the key and its action, then brought back by an undo before the
// queue-changed message, comes back unmarked.
func TestQueueSelection_UndoOfARemovalBeforeTheEventBringsNoMarkBack(t *testing.T) {
	m := newSelectionTestModel(t, alpha, bravo, charlie)

	m, remove := updateModel(t, m, keyMsg("d"))
	m = pressWithoutEvent(t, m, "x")
	msgs, _ := drain(t, remove)
	for _, msg := range msgs {
		m, _ = updateModel(t, m, msg)
	}
	assertQueue(t, m, "/b.mp3", "/c.mp3")
	m = pressWithoutEvent(t, m, "ctrl+z")

	assertRows(t, m, "Alpha", "Bravo", "Charlie")
	assertNoSelection(t, m)
}

func TestQueueSelection_AMarkDoesNotComeBackWithItsTrack(t *testing.T) {
	for _, tt := range []struct {
		name  string
		press func(t *testing.T, m Model, keys ...string) Model
	}{
		{"with the event", pressEach},
		{"without the event", pressWithoutEvent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newSelectionTestModel(t, alpha, bravo)
			m.Core.Playback().AddTracks(charlie)
			m, _ = updateModel(t, m, ServiceQueueChangedMsg{})
			m = pressEach(t, m, "j", "j", "x")
			assertRows(t, m, "Alpha", "Bravo", "Charlie ●")

			m = tt.press(t, m, "ctrl+z")
			assertRows(t, m, "Alpha", "Bravo")
			assertNoSelection(t, m)

			m = tt.press(t, m, "ctrl+y")
			assertRows(t, m, "Alpha", "Bravo", "Charlie")
			assertNoSelection(t, m)
		})
	}
}

func TestQueueSelection_AMarkIsOnOneEntryOfATrackQueuedTwice(t *testing.T) {
	m := newSelectionTestModel(t, alpha, bravo, alpha)

	m = pressEach(t, m, "d", "j", "x", "ctrl+z")

	assertRows(t, m, "Alpha", "Bravo", "Alpha ●")

	m = pressEach(t, m, "d")
	assertQueue(t, m, "/a.mp3", "/b.mp3")
}

func TestQueueSelection_TheMarkStaysOnItsTrackAsTracksAreAddedAndThePositionMoves(t *testing.T) {
	m := newSelectionTestModel(t, alpha, bravo, charlie)
	m = pressEach(t, m, "j", "x")

	m.Core.Playback().AddTracks(core.Track{Path: "/d.mp3", Title: "Delta"})
	m, _ = updateModel(t, m, ServiceQueueChangedMsg{})
	m.Core.Playback().QueueMoveTo(2)
	m, _ = updateModel(t, m, ServiceQueueChangedMsg{})

	assertRows(t, m, "Alpha", "Bravo ●", "Charlie", "Delta")
}
