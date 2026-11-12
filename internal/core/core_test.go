package core

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/state"
)

// testTick is the core's clock in tests: short enough that a test waiting for
// background work stays fast.
const testTick = 5 * time.Millisecond

// quiet is how long a test waits before concluding that nothing happens.
const quiet = 20 * testTick

// newTestCore builds a core on a mock player and a real state database in a
// temp dir. Nothing in these tests ticks the core: it runs on its own clock.
func newTestCore(t *testing.T, opts Options) (*Core, *player.Mock) {
	t.Helper()
	mock := player.NewMock()
	opts.Player = mock
	if opts.tick == 0 {
		opts.tick = testTick
	}
	if opts.State == nil {
		opts.State = openTestState(t)
	}
	if opts.musicbrainz == nil {
		opts.musicbrainz = fakeCatalogue{} // never the real MusicBrainz
	}
	if opts.listenbrainz == nil {
		opts.listenbrainz = &fakeListenBrainz{} // nor the real ListenBrainz
	}
	if opts.volumes == nil {
		// Nor the devices plugged into the machine the tests run on.
		opts.volumes = func() ([]ExportVolume, error) { return nil, nil }
	}
	c := New(opts)
	t.Cleanup(func() { c.Close() })
	return c, mock
}

func openTestState(t *testing.T) *state.Manager {
	t.Helper()
	mgr, err := state.OpenAt(filepath.Join(t.TempDir(), "waves.db"))
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	t.Cleanup(func() { mgr.Close() })
	return mgr
}

// lastfmConfigured is a [lastfm] section an account can be linked with.
var lastfmConfigured = config.LastfmConfig{APIKey: "key", APISecret: "secret"}

// linkedState returns a state holding a Last.fm session: a core configured
// for Last.fm that opens it is linked.
func linkedState(t *testing.T) *state.Manager {
	t.Helper()
	mgr := openTestState(t)
	if err := mgr.SaveLastfmSession("lowfan", "session"); err != nil {
		t.Fatalf("save session: %v", err)
	}
	return mgr
}

// waitCall returns the next call to method, failing the test if none comes.
func waitCall(t *testing.T, calls <-chan lastfmtest.Call, method string) lastfmtest.Call {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case call := <-calls:
			if call.Method == method {
				return call
			}
		case <-deadline:
			t.Fatalf("no %s call reached Last.fm", method)
		}
	}
}

// expectNoCall fails the test if method is called within the quiet period.
func expectNoCall(t *testing.T, calls <-chan lastfmtest.Call, method string) {
	t.Helper()
	deadline := time.After(quiet)
	for {
		select {
		case call := <-calls:
			if call.Method == method {
				t.Fatalf("unexpected %s call: %v", method, call.Params)
			}
		case <-deadline:
			return
		}
	}
}

// eventually waits for cond to hold, failing the test if it never does.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting until %s", what)
		}
		time.Sleep(testTick)
	}
}

// nextEvent returns the next event of type E from the core, skipping others.
func nextEvent[E Event](t *testing.T, c *Core) E {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-c.Events():
			if !ok {
				t.Fatalf("event stream closed while waiting for %T", *new(E))
			}
			if want, ok := e.(E); ok {
				return want
			}
		case <-deadline:
			t.Fatalf("no %T event", *new(E))
		}
	}
}

// expectNoEvent fails the test if an event of type E comes within the quiet
// period.
func expectNoEvent[E Event](t *testing.T, c *Core) {
	t.Helper()
	deadline := time.After(quiet)
	for {
		select {
		case e := <-c.Events():
			if unwanted, ok := e.(E); ok {
				t.Fatalf("unexpected %T: %+v", unwanted, unwanted)
			}
		case <-deadline:
			return
		}
	}
}

func TestEvents_PlaybackEventsReachTheClientAsPlainData(t *testing.T) {
	c, _ := newTestCore(t, Options{})
	pb := c.Playback()

	pb.AddTracks(Track{Path: "/music/a.flac", Title: "A"}, Track{Path: "/music/b.flac", Title: "B"})
	if e := nextEvent[QueueChanged](t, c); len(e.Tracks) != 2 || e.Tracks[1].Title != "B" {
		t.Errorf("QueueChanged = %+v, want the two tracks", e)
	}

	pb.QueueMoveTo(0)
	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	if e := nextEvent[StateChanged](t, c); e.Previous != StateStopped || e.Current != StatePlaying {
		t.Errorf("StateChanged = %+v, want stopped -> playing", e)
	}

	if err := pb.Next(); err != nil {
		t.Fatalf("next: %v", err)
	}
	if e := nextEvent[TrackChanged](t, c); e.Index != 1 || e.Current == nil || e.Current.Title != "B" {
		t.Errorf("TrackChanged = %+v, want track B at index 1", e)
	}

	pb.SetShuffle(true)
	if e := nextEvent[ModeChanged](t, c); !e.Shuffle {
		t.Errorf("ModeChanged = %+v, want shuffle on", e)
	}
}

func TestClose_StopsPlaybackAndEndsTheEventStream(t *testing.T) {
	c, mock := newTestCore(t, Options{})
	c.Playback().AddTracks(Track{Path: "/music/a.flac"})
	c.Playback().QueueMoveTo(0)
	if err := c.Playback().Play(); err != nil {
		t.Fatalf("play: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if mock.State() != player.Stopped {
		t.Errorf("player state = %v, want stopped", mock.State())
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-c.Events():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("event stream still open after Close")
		}
	}
}

// waitFailed returns the next failure of op the core reports, skipping others.
func waitFailed(t *testing.T, c *Core, op errmsg.Op) Failed {
	t.Helper()
	for {
		if e := nextEvent[Failed](t, c); e.Op == op {
			return e
		}
	}
}
