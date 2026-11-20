package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

// Run runs p until it ends, however it ends, then closes c, the core of its
// model: the quit key closes it before p ends, and Run gets the error of that
// Close; every other way out (SIGTERM, SIGINT, an error of the program) leaves
// the close to Run. SIGHUP, the terminal closed, ends p the way SIGTERM does
// instead of killing the process. Whatever the way out, the navigation state
// is saved as the model left it before the core closes. When the
// initialization of the TUI failed, the error Run returns holds a
// StartupError with its cause.
func Run(p *tea.Program, c io.Closer) error {
	// Under nohup, SIGHUP stays ignored: listening to it would undo that.
	if !signal.Ignored(syscall.SIGHUP) {
		hup := make(chan os.Signal, 1)
		signal.Notify(hup, syscall.SIGHUP)
		done := make(chan struct{})
		defer func() {
			signal.Stop(hup)
			close(done)
		}()
		go func() {
			select {
			case <-hup:
				p.Quit()
			case <-done:
			}
		}()
	}

	var errs []error
	final, err := p.Run()
	if m, ok := final.(Model); ok {
		if m.startupErr != nil {
			errs = append(errs, &StartupError{Err: m.startupErr})
		}
		// The quit key saved it before it closed the core; any other way out
		// saves it here, before Run closes the core.
		m.saveNavigationAtQuit()
	}
	if err != nil {
		errs = append(errs, fmt.Errorf("run the program: %w", err))
	}
	if cerr := c.Close(); cerr != nil {
		errs = append(errs, fmt.Errorf("close the core: %w", cerr))
	}
	return errors.Join(errs...)
}

// StartupError is a failed initialization of the TUI: the session ended
// before it started.
type StartupError struct{ Err error }

func (e *StartupError) Error() string { return e.Err.Error() }

func (e *StartupError) Unwrap() error { return e.Err }
