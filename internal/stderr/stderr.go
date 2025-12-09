// Package stderr captures stderr output from C libraries (ALSA, minimp3)
// that write directly to file descriptor 2, bypassing Go's os.Stderr.
// This prevents raw error messages from corrupting the TUI layout.
package stderr

import (
	"bufio"
	"os"
	"strings"
	"syscall"
)

// Messages receives stderr lines captured from C libraries.
// Callers should read from this channel to display errors in the UI.
var Messages = make(chan string, 100)

var (
	origStderr int
	pipeRead   *os.File
	pipeWrite  *os.File
	started    bool
)

// Start begins capturing stderr output.
// Must be called early in main(), before any C library initialization.
// Returns an error if capture cannot be set up, but the program can continue
// without stderr capture (errors will just go to the original stderr).
func Start() error {
	if started {
		return nil
	}

	// Create a pipe
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}

	// Save original stderr file descriptor
	origStderr, err = syscall.Dup(int(os.Stderr.Fd()))
	if err != nil {
		r.Close()
		w.Close()
		return err
	}

	// Redirect stderr (fd 2) to the pipe's write end
	err = syscall.Dup2(int(w.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		syscall.Close(origStderr)
		r.Close()
		w.Close()
		return err
	}

	pipeRead = r
	pipeWrite = w
	started = true

	// Start goroutine to read from pipe and send to channel
	go func() {
		scanner := bufio.NewScanner(pipeRead)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				select {
				case Messages <- line:
				default:
					// Channel full, drop message to avoid blocking
				}
			}
		}
	}()

	return nil
}

// WriteOriginal writes directly to the original stderr, bypassing capture.
// Useful for fatal errors that must be visible even if TUI is running.
func WriteOriginal(msg string) {
	if origStderr > 0 {
		_, _ = syscall.Write(origStderr, []byte(msg))
	}
}

// Stop restores the original stderr. Should be called on program exit.
func Stop() {
	if !started {
		return
	}

	// Restore original stderr
	_ = syscall.Dup2(origStderr, int(os.Stderr.Fd()))
	_ = syscall.Close(origStderr)

	// Close pipe
	pipeWrite.Close()
	pipeRead.Close()

	close(Messages)
	started = false
}
