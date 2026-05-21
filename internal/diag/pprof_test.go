// internal/diag/pprof_test.go
package diag

import (
	"net/http"
	"testing"
)

func TestMaybeStartPprof_DisabledWhenEmpty(t *testing.T) {
	started, addr, err := MaybeStartPprof("")
	if started || addr != "" || err != nil {
		t.Fatalf("empty addr: got (%v,%q,%v), want (false,\"\",nil)", started, addr, err)
	}
}

func TestMaybeStartPprof_ServesProfileIndex(t *testing.T) {
	started, addr, err := MaybeStartPprof("127.0.0.1:0")
	if err != nil || !started {
		t.Fatalf("start: got (started=%v,addr=%q,err=%v)", started, addr, err)
	}
	// Listener and goroutine are intentionally not cleaned up: MaybeStartPprof
	// has no shutdown API (YAGNI for a diagnostic tool); process exit is the
	// cleanup. Do not add a t.Cleanup here — it cannot stop the server.
	resp, err := http.Get("http://" + addr + "/debug/pprof/")
	if err != nil {
		t.Fatalf("GET pprof index: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
