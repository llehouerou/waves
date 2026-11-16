package app

import (
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/ui/testutil"
)

// newFailingStartupModel is a model that waits for its initialization, as New
// builds it, on a first launch whose default_folder does not exist: its
// initialization fails.
func newFailingStartupModel(t *testing.T) Model {
	t.Helper()
	return newWaitingModel(t, filepath.Join(t.TempDir(), "waves.db"), "/no/such/folder")
}

// failedInit runs the model's initialization and checks that it fails.
func failedInit(t *testing.T, m Model) InitResult {
	t.Helper()
	init, ok := m.startInitialization()().(InitResult)
	if !ok || !errors.Is(init.Err, fs.ErrNotExist) {
		t.Fatalf("initialization = %+v, want it to fail on the missing folder", init)
	}
	return init
}

// A startup that fails ends the session at once: no popup, the core closed,
// the program ended, and nothing reaches the views that were never built.
func TestStartupFailure_EndsTheSession(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, m Model) Model
	}{
		{name: "before the loading screen shows", setup: func(_ *testing.T, m Model) Model { return m }},
		{name: "while the loading screen shows", setup: func(t *testing.T, m Model) Model {
			m, _ = updateModel(t, m, ShowLoadingMsg{})
			if m.loadingState != loadingShowing {
				t.Fatalf("loading state = %v, want showing", m.loadingState)
			}
			return m
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newFailingStartupModel(t)
			m, _ = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
			m = tt.setup(t, m)
			init := failedInit(t, m)

			m, cmd := updateModel(t, m, init)

			if msg := m.Popups.ErrorMsg(); msg != "" {
				t.Errorf("error popup = %q, want none", msg)
			}
			view := testutil.StripANSI(m.View())
			if strings.Contains(view, "Failed to initialize application") {
				t.Errorf("the view shows the initialization error:\n%s", view)
			}
			if m.loadingState == loadingDone {
				t.Error("loading state = done, want the normal UI never shown")
			}
			if cmd == nil {
				t.Fatal("no command: the program does not end")
			}
			if msg := cmd(); msg != (tea.QuitMsg{}) {
				t.Errorf("command returned %T, want tea.QuitMsg", msg)
			}
			assertCoreClosed(t, m.Core)

			for _, msg := range []tea.Msg{
				tea.KeyMsg{Type: tea.KeyF2},
				tea.KeyMsg{Type: tea.KeyF3},
				tea.KeyMsg{Type: tea.KeyF4},
				keyMsg("h"),
				keyMsg(keyEnter),
				LibraryScanCompleteMsg{},
				LoadingTickMsg{},
			} {
				next, cmd := updateModel(t, m, msg)
				if cmd != nil {
					t.Errorf("%T %v: returned a command, want none", msg, msg)
				}
				if got := next.View(); got != m.View() {
					t.Errorf("%T %v: the view changed:\n%s", msg, msg, testutil.StripANSI(got))
				}
				m = next
			}
		})
	}
}

// Run returns the cause of a failed startup, on its own, and closes the core.
func TestRun_ReturnsTheErrorOfAFailedStartup(t *testing.T) {
	m := newFailingStartupModel(t)
	p := newHeadlessProgram(m)
	done := make(chan error, 1)
	go func() { done <- Run(p, m.Core) }()

	err := waitRun(t, done)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Run = %v, want the missing folder", err)
	}
	var startupErr *StartupError
	if !errors.As(err, &startupErr) {
		t.Errorf("Run = %v, want a StartupError", err)
	}
	assertCoreClosed(t, m.Core)
}

// A startup that succeeds, run by the program from Init, then ended on the
// quit key: Run returns no error.
func TestRun_ReturnsNilAfterAStartupThatSucceeds(t *testing.T) {
	m := newWaitingModel(t, filepath.Join(t.TempDir(), "waves.db"), t.TempDir())
	applied := make(chan InitResult, 1)
	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutSignalHandler(),
		tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
			if init, ok := msg.(InitResult); ok {
				applied <- init
			}
			return msg
		}))
	done := make(chan error, 1)
	go func() { done <- Run(p, m.Core) }()

	select {
	case init := <-applied:
		if init.Err != nil {
			t.Fatalf("initialization failed: %v", init.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the initialization did not reach the model")
	}
	p.Send(keyMsg("q")) // handled after the InitResult

	if err := waitRun(t, done); err != nil {
		t.Errorf("Run = %v, want nil", err)
	}
	assertCoreClosed(t, m.Core)
}

// A failed startup and an error of the last close are both returned.
func TestRun_ReturnsAFailedStartupWithTheErrorOfClose(t *testing.T) {
	m := newFailingStartupModel(t)
	closeErr := errors.New("disk full")
	p := newHeadlessProgram(m)
	done := make(chan error, 1)
	go func() { done <- Run(p, failingCloser{err: closeErr}) }()

	err := waitRun(t, done)

	if !errors.Is(err, fs.ErrNotExist) || !errors.Is(err, closeErr) {
		t.Errorf("Run = %v, want the missing folder and %v", err, closeErr)
	}
}

// A failed startup that arrives after the quit key is dropped: waves quits as
// asked.
func TestStartupFailure_AfterTheQuitKeyIsDropped(t *testing.T) {
	m := newFailingStartupModel(t)
	init := failedInit(t, m)
	m, _ = updateModel(t, m, keyMsg("q"))
	if !m.closing {
		t.Fatal("the quit key did not close the session")
	}

	m, cmd := updateModel(t, m, init)

	if cmd != nil {
		t.Error("the failed startup returned a command, want it dropped")
	}
	if m.startupErr != nil {
		t.Errorf("startup error = %v, want none: the quit came first", m.startupErr)
	}
}
