// internal/diag/pprof.go
//
// Opt-in runtime profiling for issue #28 diagnostics. Nothing listens unless
// WAVES_PPROF is set, so default builds carry zero overhead and no exposure.
package diag

import (
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // opt-in only; disabled unless WAVES_PPROF is set
)

// MaybeStartPprof opens a pprof HTTP server on addr in a background goroutine
// when addr is non-empty. It returns whether a listener was opened and the
// resolved address (useful when addr uses port 0). A non-empty addr that
// fails to bind returns the error without crashing the app.
func MaybeStartPprof(addr string) (started bool, actualAddr string, err error) {
	if addr == "" {
		return false, "", nil
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false, "", err
	}
	go func() { _ = http.Serve(ln, nil) }() //nolint:gosec // opt-in diagnostic server, no timeouts needed
	return true, ln.Addr().String(), nil
}
