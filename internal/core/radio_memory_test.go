package core

import (
	"testing"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
)

// Issue #315: radio's memory (the tracks it played, their artists, its recent
// seeds) belongs to one radio session. Leaving radio mode clears it, whatever
// changed the repeat mode, and a fill records its seeds only when the core
// queues its tracks.

// radioFilledOnce turns radio on, starts the last track of the queue and
// waits for the fill it starts to add a track by Codeine. It returns the path
// of that track.
func radioFilledOnce(t *testing.T, c *Core, calls <-chan lastfmtest.Call) string {
	t.Helper()
	radioOn(t, c)
	playAt(t, c, 1)
	waitCall(t, calls, similarMethod)
	waitRadioTracks(t, c)
	settle(c) // the loop has recorded what the fill added
	pb := c.Playback()
	if n := pb.QueueLen(); n != 3 {
		t.Fatalf("queue holds %d tracks after one fill, want 3", n)
	}
	return pb.QueueTracks()[2].Path
}

// expectFillAddsCodeine starts the last track of the queue and checks that
// the fill it starts adds a track by Codeine.
func expectFillAddsCodeine(t *testing.T, c *Core) {
	t.Helper()
	pb := c.Playback()
	before := pb.QueueLen()
	playAt(t, c, before-1)
	eventually(t, "radio adds a track to the queue", func() bool { return pb.QueueLen() > before })
	if added := pb.QueueTracks()[before]; added.Artist != "Codeine" {
		t.Errorf("radio added %q by %q, want a track by Codeine", added.Title, added.Artist)
	}
}

func TestRadio_LeavingRadioByAnyPathForgetsItsMemory(t *testing.T) {
	tests := []struct {
		name  string
		leave func(*Core, *fakeMPRIS)
		enter func(*testing.T, *Core, *fakeMPRIS)
	}{
		{
			name:  "R",
			leave: func(c *Core, _ *fakeMPRIS) { c.Playback().CycleRepeatMode() },
			enter: func(t *testing.T, c *Core, _ *fakeMPRIS) { radioOn(t, c) },
		},
		{
			name:  "the core's playback",
			leave: func(c *Core, _ *fakeMPRIS) { c.Playback().SetRepeatMode(RepeatOff) },
			enter: func(_ *testing.T, c *Core, _ *fakeMPRIS) { c.Playback().SetRepeatMode(RepeatRadio) },
		},
		{
			name:  "the playback service",
			leave: func(c *Core, _ *fakeMPRIS) { c.playback.svc.SetRepeatMode(RepeatOff) },
			enter: func(_ *testing.T, c *Core, _ *fakeMPRIS) { c.playback.svc.SetRepeatMode(RepeatRadio) },
		},
		{
			name:  "MPRIS",
			leave: func(_ *Core, bus *fakeMPRIS) { bus.served.SetRepeatMode(RepeatOff) },
			enter: func(_ *testing.T, _ *Core, bus *fakeMPRIS) { bus.served.SetRepeatMode(RepeatRadio) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := lastfmtest.Serve(t, codeineIsSimilar)
			bus := &fakeMPRIS{}
			c, _ := newRadioCoreWith(t, Options{MPRIS: true, mpris: bus.serve})
			added := radioFilledOnce(t, c, calls)

			tt.leave(c, bus)
			settle(c) // the core has seen the change

			if c.radio.IsRecentlyPlayed(added) {
				t.Errorf("%s is still recently played after radio was left", added)
			}

			tt.enter(t, c, bus)
			settle(c)

			expectFillAddsCodeine(t, c)
		})
	}
}

func TestRadio_LeavingARadioModeRestoredAtStartupForgetsItsMemory(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	database := radioDatabase(t, linkedState(t), RepeatRadio)
	c, _ := newTestCore(t, Options{
		Database: database,
		Lastfm:   lastfmConfigured,
		Radio:    (&config.Config{}).GetRadioConfig(),
	})
	playAt(t, c, 1)
	waitCall(t, calls, similarMethod)
	waitRadioTracks(t, c)
	settle(c)
	added := c.Playback().QueueTracks()[2].Path

	if mode := c.Playback().CycleRepeatMode(); mode != RepeatOff {
		t.Fatalf("cycle from the restored radio mode = %v, want Off", mode)
	}
	settle(c)

	if c.radio.IsRecentlyPlayed(added) {
		t.Errorf("%s is still recently played after radio was left", added)
	}
}

func TestRadio_FillThatLandsAfterRadioOffRecordsNoSeed(t *testing.T) {
	c, _, release := heldRadioCore(t)

	c.Playback().CycleRepeatMode() // radio -> off
	release()
	settle(c) // the fill landed, dropped
	if n := c.Playback().QueueLen(); n != 2 {
		t.Fatalf("queue holds %d tracks, want 2: the fill is dropped", n)
	}

	radioOn(t, c)
	settle(c)

	expectFillAddsCodeine(t, c)
}

func TestRadio_FillThatLandsInRadioModeRecordsItsSeeds(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, _ := newRadioCore(t)
	radioFilledOnce(t, c, calls)

	// The seed is Codeine, the artist of the track playing, a recent seed:
	// nothing to add.
	playAt(t, c, 2)
	waitCall(t, calls, similarMethod)
	settle(c)

	if n := c.Playback().QueueLen(); n != 3 {
		t.Errorf("queue holds %d tracks, want 3: Codeine is a recent seed", n)
	}
}

func TestRadio_ModeChangeInRadioModeKeepsItsMemory(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Core)
	}{
		{"shuffle on", func(c *Core) { c.Playback().ToggleShuffle() }},
		{"shuffle on and off", func(c *Core) { c.Playback().ToggleShuffle(); c.Playback().ToggleShuffle() }},
		{"radio again", func(c *Core) { c.Playback().SetRepeatMode(RepeatRadio) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := lastfmtest.Serve(t, codeineIsSimilar)
			c, _ := newRadioCore(t)
			added := radioFilledOnce(t, c, calls)

			tt.change(c)
			settle(c)

			if !c.radio.IsRecentlyPlayed(added) {
				t.Errorf("%s is no longer recently played", added)
			}
		})
	}
}
