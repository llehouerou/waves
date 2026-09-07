package playback

import (
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/playlist"
)

// Opening a track is file I/O and can take seconds, or hang when a network mount
// is unreachable. The service must stay readable while it happens: the UI calls
// State/Position/CurrentTrack on every frame, and blocking those freezes the
// whole interface (issue #45).
func TestService_StaysReadableWhileTrackIsOpening(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
	q.JumpTo(0)

	svc := New(p, q)
	defer svc.Close()
	if err := svc.Play(); err != nil {
		t.Fatal(err)
	}

	// The next track will hang while opening.
	release := p.BlockPlay()
	defer release()
	p.SimulateFinished()

	// Wait until the service is inside the blocked Play.
	deadline := time.Now().Add(2 * time.Second)
	for len(p.PlayCalls()) < 2 {
		if time.Now().After(deadline) {
			t.Fatal("the service never reached the blocked Play")
		}
		time.Sleep(time.Millisecond)
	}

	// What the renderer does every frame.
	done := make(chan struct{})
	go func() {
		svc.State()
		svc.Position()
		svc.Duration()
		svc.CurrentTrack()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("UI reads blocked while a track was opening: the interface would freeze")
	}
}

// Same for a user-initiated command: pressing next on a slow mount must not
// freeze the interface either.
func TestService_StaysReadableWhileNextIsOpening(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(playlist.Track{Path: testSvcPathA}, playlist.Track{Path: testSvcPathB})
	q.JumpTo(0)

	svc := New(p, q)
	defer svc.Close()
	if err := svc.Play(); err != nil { // Next only restarts an active player
		t.Fatal(err)
	}

	release := p.BlockPlay()
	defer release()

	go func() { _ = svc.Next() }()

	deadline := time.Now().Add(2 * time.Second)
	for len(p.PlayCalls()) < 2 {
		if time.Now().After(deadline) {
			t.Fatal("Next never reached the blocked Play")
		}
		time.Sleep(time.Millisecond)
	}

	done := make(chan struct{})
	go func() {
		svc.State()
		svc.CurrentTrack()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("UI reads blocked while Next was opening a track")
	}
}

// Commands issued while another is opening must serialise, not deadlock.
func TestService_ConcurrentCommandsDoNotDeadlock(t *testing.T) {
	p := player.NewMock()
	q := playlist.NewQueue()
	q.Add(
		playlist.Track{Path: testSvcPathA},
		playlist.Track{Path: testSvcPathB},
		playlist.Track{Path: testSvcPathC},
	)
	q.JumpTo(0)

	svc := New(p, q)
	defer svc.Close()

	release := p.BlockPlay()

	done := make(chan struct{})
	go func() {
		_ = svc.Play()
		_ = svc.Next()
		_ = svc.Previous()
		_ = svc.Stop()
		close(done)
	}()

	time.Sleep(10 * time.Millisecond) // let the first command block on Play
	release()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("commands deadlocked")
	}
}
