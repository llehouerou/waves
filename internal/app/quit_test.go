package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/testutil"
)

// holdFirstTrack makes the file of the first track of Low's Trust a FIFO: an
// export copying it lasts until the end the test writes at is closed.
func holdFirstTrack(t *testing.T, m Model) (path string) {
	t.Helper()
	tracks, err := m.Core.Library().Tracks("Low", "Trust")
	if err != nil || len(tracks) == 0 {
		t.Fatalf("tracks = %+v, %v", tracks, err)
	}
	path = tracks[0].Path
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	return path
}

// feedTrack waits for an export to be copying the track at path, a FIFO, and
// returns the end the test writes it at.
func feedTrack(t *testing.T, path string) *os.File {
	t.Helper()
	opened := make(chan *os.File, 1)
	go func() {
		// Opening this end returns once the export has opened the other.
		w, _ := os.OpenFile(path, os.O_WRONLY, 0) //nolint:errcheck // a nil file fails the test below
		opened <- w
	}()
	select {
	case w := <-opened:
		if w == nil {
			t.Fatal("cannot open the held track")
		}
		// Registered after the core's: a failed test must not leave Close
		// waiting for a copy nobody ends.
		t.Cleanup(func() { w.Close() })
		return w
	case <-time.After(5 * time.Second):
		t.Fatal("no export copies the held track")
		return nil
	}
}

// coreClosed reports whether the core has closed, draining what it emitted.
func coreClosed(c *core.Core) bool {
	for {
		select {
		case _, open := <-c.Events():
			if !open {
				return true
			}
		default:
			return false
		}
	}
}

// The quit key answers at once: the core finishes its background work while
// the screen says so, and the program ends once the core is closed.
func TestQuit_SaysItFinishesTheBackgroundWorkAndEndsOnceTheCoreClosed(t *testing.T) {
	for _, key := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("q")}, {Type: tea.KeyCtrlC}} {
		t.Run(key.String(), func(t *testing.T) {
			m, dest := newExportTestModel(t, false)
			held := holdFirstTrack(t, m)
			m = press(t, m, "e")
			m = press(t, m, keyEnter)
			w := feedTrack(t, held)
			before := testutil.StripANSI(m.View())

			type updated struct {
				m   tea.Model
				cmd tea.Cmd
			}
			answered := make(chan updated, 1)
			go func() {
				next, cmd := m.Update(key)
				answered <- updated{next, cmd}
			}()
			var u updated
			select {
			case u = <-answered:
			case <-time.After(5 * time.Second):
				w.Close()
				t.Fatal("the quit key waits for the export")
			}
			closing, ok := u.m.(Model)
			if !ok {
				t.Fatalf("Update returned %T, want Model", u.m)
			}
			quit := u.cmd
			if quit == nil {
				t.Fatal("the quit key returns no command")
			}
			if coreClosed(m.Core) {
				t.Fatal("the core is closed before the command runs")
			}

			view := testutil.StripANSI(closing.View())
			if !testutil.ContainsLine(view, "Finishing the background work before quitting") {
				t.Errorf("no line says waves finishes its background work:\n%s", view)
			}
			// The header, then the navigator and the queue, under their titles.
			header, _, _ := strings.Cut(before, "\n")
			titles := testutil.FindLine(before, "Queue (")
			if !strings.HasPrefix(view, header+"\n") || titles == "" || !strings.Contains(view, titles+"\n") {
				t.Errorf("the panels are gone; before:\n%s\nnow:\n%s", before, view)
			}

			ended := make(chan tea.Msg, 1)
			go func() { ended <- quit() }()

			// Meanwhile, nothing changes the model.
			for _, msg := range []tea.Msg{
				key,
				keyMsg("j"),
				keyMsg(" "),
				tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown},
				TickMsg{Gen: closing.tickGen},
				ServiceExportMsg{Event: core.ExportProgressed{Target: "Backup", Total: 3}},
			} {
				next, cmd := updateModel(t, closing, msg)
				if cmd != nil {
					t.Errorf("%T %v while closing returns a command", msg, msg)
				}
				if !reflect.DeepEqual(next, closing) {
					t.Errorf("%T %v while closing changes the model", msg, msg)
				}
				if e := next.Popups.ErrorMsg(); e != "" {
					t.Errorf("%T %v while closing opens an error popup: %s", msg, msg, e)
				}
			}

			select {
			case msg := <-ended:
				t.Fatalf("the command returned %T while the export still copies", msg)
			case <-time.After(100 * time.Millisecond):
			}

			if _, err := w.WriteString("track 1"); err != nil {
				t.Fatalf("write the held track: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("close the held track: %v", err)
			}
			select {
			case msg := <-ended:
				if _, ok := msg.(tea.QuitMsg); !ok {
					t.Errorf("the command returned %T, want tea.QuitMsg", msg)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the command does not end once the export did")
			}
			if !coreClosed(m.Core) {
				t.Error("the core is still open when the program ends")
			}
			copied := filepath.Join(dest, "Low - Trust", "01 - Track 1.mp3")
			if got, err := os.ReadFile(copied); err != nil || string(got) != "track 1" {
				t.Errorf("%s = %q (err %v), want the track whole", copied, got, err)
			}
		})
	}
}

// A quit key pressed while waves still starts has no notification bar to
// show its line in: the loading screen gets it.
func TestQuit_DuringStartupSaysItFinishesTheBackgroundWork(t *testing.T) {
	m := newIntegrationTestModel(t)
	m.loadingState = loadingWaiting

	closing, quit := updateModel(t, m, keyMsg("q"))
	if quit == nil {
		t.Fatal("the quit key returns no command")
	}
	if view := testutil.StripANSI(closing.View()); !testutil.ContainsLine(view, "Finishing the background work before quitting") {
		t.Errorf("no line says waves finishes its background work:\n%s", view)
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Error("the quit command does not end the program")
	}
}
