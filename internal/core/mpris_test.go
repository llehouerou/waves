package core

import (
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/llehouerou/waves/internal/mpris"
	"github.com/llehouerou/waves/internal/playback"
)

// fakeMPRIS stands in for the D-Bus server: it keeps the playback and the
// volume it was given to serve, how many times it was told the volume
// changed, and what it saw of playback when it was taken off the bus.
type fakeMPRIS struct {
	mu             sync.Mutex
	served         playback.Service
	volume         mpris.Volume
	volumeChanges  int
	closed         bool
	playingAtClose bool
}

func (f *fakeMPRIS) serve(svc playback.Service, volume mpris.Volume) mprisServer {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.served = svc
	f.volume = volume
	return f
}

func (f *fakeMPRIS) VolumeChanged() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.volumeChanges++
}

func (f *fakeMPRIS) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	f.playingAtClose = f.served.IsPlaying()
	return nil
}

func TestMPRIS_IsServedWithNoClientDrivingIt(t *testing.T) {
	bus := &fakeMPRIS{}
	c, _ := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
	c.Playback().AddTracks(Track{Path: "/music/a.flac", Title: "A"}, Track{Path: "/music/b.flac", Title: "B"})
	c.Playback().QueueMoveTo(0)
	if bus.served == nil {
		t.Fatal("the core serves no MPRIS")
	}

	// What a desktop widget sends: no client of the core is involved.
	if err := bus.served.Play(); err != nil {
		t.Fatalf("play over MPRIS: %v", err)
	}
	if err := bus.served.Next(); err != nil {
		t.Fatalf("next over MPRIS: %v", err)
	}

	if track := c.Playback().CurrentTrack(); !c.Playback().IsPlaying() || track == nil || track.Title != "B" {
		t.Errorf("the core plays %+v (playing %v), want B", track, c.Playback().IsPlaying())
	}
	// And the core reports it as it does what a client asked for.
	if e := nextEvent[TrackChanged](t, c); e.Current == nil || e.Current.Title != "B" {
		t.Errorf("TrackChanged = %+v, want track B", e)
	}
}

// A core nobody asked to serve MPRIS must not register on the session bus:
// the tests of a client build theirs this way, on a machine where a waves
// may be playing.
func TestMPRIS_IsNotServedUnlessAskedFor(t *testing.T) {
	bus := &fakeMPRIS{}
	c, _ := newTestCore(t, Options{mpris: bus.serve})

	if bus.served != nil || c.mpris != nil {
		t.Error("the core serves MPRIS, and was not asked to")
	}
}

// A widget told that playback stopped offers to start it again, on a core
// that is closing: MPRIS leaves the bus before anything else ends.
func TestClose_TakesMPRISOffTheBusBeforePlaybackStops(t *testing.T) {
	bus := &fakeMPRIS{}
	c, _ := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
	c.Playback().AddTracks(Track{Path: "/music/a.flac"})
	c.Playback().QueueMoveTo(0)
	if err := c.Playback().Play(); err != nil {
		t.Fatalf("play: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if !bus.closed {
		t.Fatal("Close left MPRIS on the bus")
	}
	if !bus.playingAtClose {
		t.Error("playback had stopped when MPRIS left the bus")
	}
}

// MPRIS was the last thing outside the core to listen to the playback service
// itself. A client has Core.Events, and neither closes playback nor restores
// a queue: the core does.
func TestPlayback_GivesAClientNoWayAroundTheCore(t *testing.T) {
	c, _ := newTestCore(t, Options{})
	facade := reflect.TypeOf(c.Playback())

	for _, name := range []string{"Subscribe", "Close", "RestoreQueue"} {
		if _, ok := facade.MethodByName(name); ok {
			t.Errorf("core.Playback exposes %s", name)
		}
	}
}

// MPRIS has no mute: a muted waves reports what is heard.
func TestMPRIS_ReportsTheVolumeThatIsHeard(t *testing.T) {
	bus := &fakeMPRIS{}
	c, _ := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
	pb := c.Playback()
	for range 3 {
		pb.AdjustVolume(-0.10)
	}

	if got := bus.volume.Volume(); got != 0.7 {
		t.Errorf("MPRIS volume = %v, want 0.7", got)
	}
	pb.ToggleMute()
	if got := bus.volume.Volume(); got != 0 {
		t.Errorf("MPRIS volume muted = %v, want 0", got)
	}
}

func TestMPRIS_SetsTheVolumeAndUnmutes(t *testing.T) {
	database := filepath.Join(t.TempDir(), "waves.db")
	bus := &fakeMPRIS{}
	c, _ := newTestCore(t, Options{MPRIS: true, mpris: bus.serve, Database: database})
	c.Playback().ToggleMute()

	// What a desktop widget sends: no client of the core is involved.
	bus.volume.SetVolume(0.4)

	if got := c.Playback().Volume(); got != 0.4 || c.Playback().Muted() {
		t.Errorf("volume = %v muted = %v, want 0.4 unmuted", got, c.Playback().Muted())
	}
	// The client did not send it: it is told.
	nextEvent[VolumeChanged](t, c)
	expectNoEvent[VolumeChanged](t, c)
	eventually(t, "the volume is saved", func() bool {
		saved, err := c.state.GetVolume()
		if err != nil {
			t.Fatalf("get volume: %v", err)
		}
		return saved.Volume == 0.4 && !saved.Muted
	})
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, _ := newTestCore(t, Options{Database: database})
	if got := second.Playback().Volume(); got != 0.4 || second.Playback().Muted() {
		t.Errorf("restored volume = %v muted = %v, want 0.4 unmuted", got, second.Playback().Muted())
	}
}

// A client that changes the volume gets no event, as for every command it
// sends; the widgets on the bus are told.
func TestMPRIS_IsToldOfEveryVolumeChange(t *testing.T) {
	bus := &fakeMPRIS{}
	c, _ := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})

	c.Playback().AdjustVolume(-0.10)
	c.Playback().ToggleMute()
	c.Playback().ToggleMute()

	if bus.volumeChanges != 3 {
		t.Errorf("MPRIS told of %d volume changes, want 3", bus.volumeChanges)
	}
	expectNoEvent[VolumeChanged](t, c)
}
