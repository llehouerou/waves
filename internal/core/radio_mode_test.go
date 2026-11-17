package core

import (
	"errors"
	"net/url"
	"testing"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/state"
)

// Issue #197: radio mode needs Last.fm configured, not a linked account. The
// repeat mode is Radio only on a core that has a radio.

// lowQueue is the queue an earlier run left: Low's two tracks of the library
// addLibraryTracks fills, on the second, shuffled, in mode.
func lowQueue(mode RepeatMode) state.QueueState {
	return state.QueueState{
		CurrentIndex: 1,
		RepeatMode:   int(mode),
		Shuffle:      true,
		Tracks: []state.QueueTrack{
			{Path: libraryPath("Low", 1), Artist: "Low", Title: "Low 1"},
			{Path: libraryPath("Low", 2), Artist: "Low", Title: "Low 2"},
		},
	}
}

// radioDatabase seeds db with a library holding two tracks by Low and three
// by Codeine, and Low's queue saved in mode, then closes it and returns its
// path.
func radioDatabase(t *testing.T, db *testDatabase, mode RepeatMode) string {
	t.Helper()
	addLibraryTracks(t, db.DB(), "Low", 2)
	addLibraryTracks(t, db.DB(), "Codeine", 3)
	if err := db.SaveQueue(lowQueue(mode)); err != nil {
		t.Fatalf("save queue: %v", err)
	}
	return db.closed(t)
}

// savedRepeatMode reads the repeat mode saved in the database at path, which
// no core holds.
func savedRepeatMode(t *testing.T, path string) RepeatMode {
	t.Helper()
	mgr, err := state.OpenAt(path)
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	defer mgr.Close()
	return RepeatMode(savedQueue(t, mgr).RepeatMode)
}

func TestRadioMode_StaysOnAfterTheAccountIsUnlinked(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, _ := newRadioCore(t)
	radioOn(t, c)

	if err := c.Lastfm().Unlink(); err != nil {
		t.Fatalf("unlink: %v", err)
	}

	if mode := c.Playback().RepeatMode(); mode != RepeatRadio {
		t.Fatalf("repeat mode = %v after the unlink, want Radio", mode)
	}
	playAt(t, c, 1)
	waitCall(t, calls, similarMethod)
	waitRadioTracks(t, c)

	if mode := c.Playback().CycleRepeatMode(); mode != RepeatOff {
		t.Fatalf("cycle from radio = %v, want Off", mode)
	}
	radioOn(t, c) // three more presses reach it again
}

func TestRadioMode_StaysOnWhenLastfmRefusesTheSession(t *testing.T) {
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		if method == similarMethod {
			return codeineIsSimilar(method, params)
		}
		return sessionRevoked(method, params)
	})
	seeded := linkedState(t)
	addPending(t, seeded.Manager, "Lullaby")
	database := radioDatabase(t, seeded, RepeatRadio)
	c, _ := newTestCore(t, Options{
		Database: database,
		Lastfm:   lastfmConfigured,
		Radio:    (&config.Config{}).GetRadioConfig(),
	})

	if e := waitFailed(t, c, errmsg.OpLastfmAuth); !errors.Is(e.Err, errLastfmSessionLost) {
		t.Errorf("Failed = %+v, want the session Last.fm no longer accepts", e)
	}
	if _, linked := c.Lastfm().Account(); linked {
		t.Fatal("the account is still linked")
	}

	if mode := c.Playback().RepeatMode(); mode != RepeatRadio {
		t.Fatalf("repeat mode = %v after the session was lost, want Radio", mode)
	}
	playAt(t, c, 1)
	waitCall(t, calls, similarMethod)
	waitRadioTracks(t, c)
}

func TestRadioMode_IsRestoredWithNoAccount(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	database := radioDatabase(t, openTestState(t), RepeatRadio)
	c, _ := newTestCore(t, Options{
		Database: database,
		Lastfm:   lastfmConfigured,
		Radio:    (&config.Config{}).GetRadioConfig(),
	})

	if mode := c.Playback().RepeatMode(); mode != RepeatRadio {
		t.Fatalf("repeat mode = %v, want Radio", mode)
	}
	playAt(t, c, 1)
	waitCall(t, calls, similarMethod)
	waitRadioTracks(t, c)
	expectNoEvent[RadioTurnedOff](t, c)
}

func TestRadioMode_RestoredWithNoLastfmConfigIsTurnedOffAndTold(t *testing.T) {
	database := radioDatabase(t, openTestState(t), RepeatRadio)
	c, _ := newTestCore(t, Options{Database: database})
	pb := c.Playback()

	if mode := pb.RepeatMode(); mode != RepeatOff {
		t.Errorf("repeat mode = %v, want Off: the core has no radio", mode)
	}
	tracks := pb.QueueTracks()
	if len(tracks) != 2 || tracks[0].Title != "Low 1" || tracks[1].Title != "Low 2" {
		t.Errorf("tracks = %+v, want Low's two tracks", tracks)
	}
	if pb.QueueCurrentIndex() != 1 || !pb.Shuffle() {
		t.Errorf("index %d, shuffle %v, want 1 and shuffled", pb.QueueCurrentIndex(), pb.Shuffle())
	}
	if n := count[RadioTurnedOff](eventsDuring(c)); n != 1 {
		t.Errorf("%d RadioTurnedOff, want one", n)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if mode := savedRepeatMode(t, database); mode != RepeatOff {
		t.Fatalf("saved repeat mode = %v, want Off", mode)
	}

	for _, cfg := range []config.LastfmConfig{lastfmConfigured, {}} {
		next, _ := newTestCore(t, Options{Database: database, Lastfm: cfg})
		if mode := next.Playback().RepeatMode(); mode != RepeatOff {
			t.Errorf("Last.fm %+v: the next run starts in %v, want Off", cfg, mode)
		}
		expectNoEvent[RadioTurnedOff](t, next)
		if err := next.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}

func TestRadioMode_OtherModesAreRestoredWithNoLastfmConfig(t *testing.T) {
	for _, mode := range []RepeatMode{RepeatOff, RepeatAll, RepeatOne} {
		t.Run(mode.String(), func(t *testing.T) {
			database := radioDatabase(t, openTestState(t), mode)
			c, _ := newTestCore(t, Options{Database: database})

			if got := c.Playback().RepeatMode(); got != mode {
				t.Errorf("repeat mode = %v, want %v", got, mode)
			}
			expectNoEvent[RadioTurnedOff](t, c)
		})
	}
}

func TestRadioMode_SetRepeatModeNeedsARadio(t *testing.T) {
	c, _ := newTestCore(t, Options{})
	c.Playback().SetRepeatMode(RepeatAll)
	c.Playback().SetRepeatMode(RepeatRadio)
	if mode := c.Playback().RepeatMode(); mode != RepeatAll {
		t.Errorf("no Last.fm: repeat mode = %v, want All, as it was", mode)
	}

	configured, _ := newTestCore(t, Options{Lastfm: lastfmConfigured})
	configured.Playback().SetRepeatMode(RepeatRadio)
	if mode := configured.Playback().RepeatMode(); mode != RepeatRadio {
		t.Errorf("Last.fm configured: repeat mode = %v, want Radio", mode)
	}
}
