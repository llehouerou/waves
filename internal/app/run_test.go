package app

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
)

// newHeadlessProgram runs m with no terminal: no input, no output, no signal
// handler of Bubble Tea's own.
func newHeadlessProgram(m Model) *tea.Program {
	return tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutSignalHandler())
}

// startRun runs p through Run and returns once its event loop takes messages.
func startRun(p *tea.Program, c io.Closer) <-chan error {
	done := make(chan error, 1)
	go func() { done <- Run(p, c) }()
	p.Send(tea.WindowSizeMsg{Width: 80, Height: 24}) // returns once the loop runs
	return done
}

func waitRun(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
		return nil
	}
}

func assertCoreClosed(t *testing.T, c *core.Core) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-c.Events():
			if !ok {
				return
			}
		case <-timeout:
			t.Fatal("the core's events are still open: the core was not closed")
		}
	}
}

// However the program ends, Run closes the core: not only the quit key does.
func TestRun_ClosesTheCoreHoweverTheProgramEnds(t *testing.T) {
	tests := []struct {
		name    string
		msg     tea.Msg
		wantErr error
	}{
		{name: "quit key", msg: keyMsg("q")},
		{name: "quit from outside the model (SIGTERM)", msg: tea.QuitMsg{}},
		{name: "interrupt (SIGINT)", msg: tea.InterruptMsg{}, wantErr: tea.ErrInterrupted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newIntegrationTestModel()
			p := newHeadlessProgram(m)
			done := startRun(p, m.Core)

			p.Send(tt.msg)

			err := waitRun(t, done)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Run = %v, want %v", err, tt.wantErr)
			}
			assertCoreClosed(t, m.Core)
		})
	}
}

// Closing the terminal (SIGHUP) ends the program the way SIGTERM does,
// instead of killing the process with the core open.
func TestRun_SIGHUPEndsTheProgramAndClosesTheCore(t *testing.T) {
	if signal.Ignored(syscall.SIGHUP) {
		t.Skip("SIGHUP is ignored (nohup): Run leaves it so")
	}
	m := newIntegrationTestModel()
	p := newHeadlessProgram(m)
	done := startRun(p, m.Core)

	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatalf("send SIGHUP: %v", err)
	}

	if err := waitRun(t, done); err != nil {
		t.Errorf("Run = %v, want nil", err)
	}
	assertCoreClosed(t, m.Core)
}

type failingCloser struct{ err error }

func (f failingCloser) Close() error { return f.err }

// An error of the last close is returned, not swallowed.
func TestRun_ReturnsTheErrorOfClose(t *testing.T) {
	m := newIntegrationTestModel()
	t.Cleanup(func() { _ = m.Core.Close() })
	closeErr := errors.New("disk full")
	p := newHeadlessProgram(m)
	done := startRun(p, failingCloser{err: closeErr})

	p.Send(tea.QuitMsg{})

	if err := waitRun(t, done); !errors.Is(err, closeErr) {
		t.Errorf("Run = %v, want %v", err, closeErr)
	}
}
