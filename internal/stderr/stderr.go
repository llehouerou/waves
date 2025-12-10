// Package stderr captures stderr output from C libraries
// that write directly to file descriptor 2, bypassing Go's os.Stderr.
// This prevents raw error messages from corrupting the TUI layout.
package stderr

// Messages receives stderr lines captured from C libraries.
// Callers should read from this channel to display errors in the UI.
var Messages = make(chan string, 100)
