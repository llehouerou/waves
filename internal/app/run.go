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
// model: the quit key closes it first, every other way out (SIGTERM, SIGINT,
// an error of the program) leaves it to Run. SIGHUP, the terminal closed,
// ends p the way SIGTERM does instead of killing the process.
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

	_, err := p.Run()
	if err != nil {
		err = fmt.Errorf("run the program: %w", err)
	}
	if cerr := c.Close(); cerr != nil {
		err = errors.Join(err, fmt.Errorf("close the core: %w", cerr))
	}
	return err
}
