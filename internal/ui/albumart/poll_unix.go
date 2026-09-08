//go:build unix

package albumart

import (
	"io"
	"time"

	"golang.org/x/sys/unix"
)

// waitReadable reports whether the descriptor has data to read before the
// timeout expires.
func waitReadable(fd uintptr, timeout time.Duration) bool {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}} //nolint:gosec // fd fits in int32
	for {
		n, err := unix.Poll(fds, int(timeout.Milliseconds()))
		if err == unix.EINTR {
			continue
		}
		return err == nil && n > 0
	}
}

// deadlineReader reads from a raw descriptor, giving up when the deadline
// passes. os.File deadlines are unusable here: Go keeps stdin in blocking mode,
// so SetReadDeadline fails with ErrNoDeadline and a plain Read would hang until
// the user hits a key.
type deadlineReader struct {
	fd       uintptr
	deadline time.Time
}

func newDeadlineReader(fd uintptr, timeout time.Duration) io.Reader {
	return &deadlineReader{fd: fd, deadline: time.Now().Add(timeout)}
}

func (d *deadlineReader) Read(p []byte) (int, error) {
	remaining := time.Until(d.deadline)
	if remaining <= 0 || !waitReadable(d.fd, remaining) {
		return 0, io.EOF
	}
	//nolint:gosec // fd comes from os.File.Fd
	return unix.Read(int(d.fd), p)
}
