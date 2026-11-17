package core

import (
	"errors"
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

// A track that fails to open on a start sent over MPRIS is a failure no
// client sent: the core tells them, and the bus still gets the error.
func TestMPRIS_StartThatFailsToOpenIsReportedToTheClients(t *testing.T) {
	for name, start := range map[string]func(playback.Service) error{
		"Play":   playback.Service.Play,
		"Toggle": playback.Service.Toggle,
	} {
		t.Run(name, func(t *testing.T) {
			bus := &fakeMPRIS{}
			c, mock := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
			c.Playback().AddTracks(Track{Path: "/music/a.flac"}, Track{Path: "/music/b.flac"})
			c.Playback().QueueMoveTo(1)
			unreadable := errors.New("unreadable")
			mock.SetPlayError(unreadable)

			if err := start(bus.served); !errors.Is(err, unreadable) {
				t.Fatalf("%s over MPRIS = %v, want the open error", name, err)
			}

			events := eventsDuring(c)
			if n := count[PlaybackFailed](events); n != 1 {
				t.Fatalf("%d PlaybackFailed, want 1", n)
			}
			for _, e := range events {
				if failed, ok := e.(PlaybackFailed); ok {
					if failed.Path != "/music/b.flac" || !errors.Is(failed.Err, unreadable) || failed.Operation != "play" {
						t.Errorf("PlaybackFailed = %+v, want play of /music/b.flac failing with the open error", failed)
					}
				}
			}
			if n := count[StateChanged](events) + count[TrackChanged](events); n != 0 {
				t.Errorf("%d StateChanged or TrackChanged, want none", n)
			}
			if !c.Playback().IsStopped() || c.Playback().QueueCurrentIndex() != 1 {
				t.Errorf("stopped %v at %d, want stopped at 1", c.Playback().IsStopped(), c.Playback().QueueCurrentIndex())
			}
		})
	}
}

// The path is the track MPRIS asked to start, even when a client moves the
// queue while the file is being opened.
func TestMPRIS_StartThatFailsToOpenNamesTheTrackItTried(t *testing.T) {
	bus := &fakeMPRIS{}
	c, mock := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
	c.Playback().AddTracks(Track{Path: "/music/a.flac"}, Track{Path: "/music/b.flac"})
	c.Playback().QueueMoveTo(1)
	mock.SetPlayError(errors.New("unreadable"))
	release := mock.BlockPlay()

	played := make(chan error, 1)
	go func() { played <- bus.served.Play() }()
	eventually(t, "the file is being opened", func() bool { return len(mock.PlayCalls()) == 1 })
	c.Playback().QueueMoveTo(0)
	release()
	if err := <-played; err == nil {
		t.Fatal("play over MPRIS succeeded, want the open failure")
	}

	if e := nextEvent[PlaybackFailed](t, c); e.Path != "/music/b.flac" {
		t.Errorf("PlaybackFailed.Path = %q, want /music/b.flac", e.Path)
	}
}

// With no track to start, nothing failed to open: the MPRIS adapter gets the
// refusal (and has no effect on the bus), the clients are told nothing.
func TestMPRIS_StartWithNoTrackIsNotAFailedPlay(t *testing.T) {
	bus := &fakeMPRIS{}
	c, _ := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})

	if err := bus.served.Play(); !errors.Is(err, playback.ErrEmptyQueue) {
		t.Errorf("Play over MPRIS = %v, want ErrEmptyQueue", err)
	}
	if err := bus.served.Toggle(); !errors.Is(err, playback.ErrEmptyQueue) {
		t.Errorf("Toggle over MPRIS = %v, want ErrEmptyQueue", err)
	}
	expectNoEvent[PlaybackFailed](t, c)
}

// A start over MPRIS on a queue with tracks and no current track plays its
// first track.
func TestMPRIS_StartWithNoCurrentTrackPlaysTheFirstTrack(t *testing.T) {
	for name, start := range map[string]func(playback.Service) error{
		"Play":   playback.Service.Play,
		"Toggle": playback.Service.Toggle,
	} {
		t.Run(name, func(t *testing.T) {
			bus := &fakeMPRIS{}
			c, mock := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
			c.Playback().AddTracks(Track{Path: "/music/a.flac"}, Track{Path: "/music/b.flac"})

			if err := start(bus.served); err != nil {
				t.Fatalf("%s over MPRIS = %v, want nil", name, err)
			}
			if calls := mock.PlayCalls(); len(calls) != 1 || calls[0] != "/music/a.flac" {
				t.Errorf("PlayCalls = %v, want [/music/a.flac]", calls)
			}
			if !c.Playback().IsPlaying() || c.Playback().QueueCurrentIndex() != 0 {
				t.Errorf("playing %v at %d, want playing at 0", c.Playback().IsPlaying(), c.Playback().QueueCurrentIndex())
			}
			expectNoEvent[PlaybackFailed](t, c)
		})
	}
}

// The first track of a queue with no current track that fails to open on a
// start over MPRIS is reported to the clients, by its path.
func TestMPRIS_StartOfAFirstTrackThatFailsToOpenNamesIt(t *testing.T) {
	for name, start := range map[string]func(playback.Service) error{
		"Play":   playback.Service.Play,
		"Toggle": playback.Service.Toggle,
	} {
		t.Run(name, func(t *testing.T) {
			bus := &fakeMPRIS{}
			c, mock := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
			c.Playback().AddTracks(Track{Path: "/music/a.flac"}, Track{Path: "/music/b.flac"})
			unreadable := errors.New("unreadable")
			mock.SetPlayError(unreadable)

			if err := start(bus.served); !errors.Is(err, unreadable) {
				t.Fatalf("%s over MPRIS = %v, want the open error", name, err)
			}

			events := eventsDuring(c)
			if n := count[PlaybackFailed](events); n != 1 {
				t.Fatalf("%d PlaybackFailed, want 1", n)
			}
			for _, e := range events {
				if failed, ok := e.(PlaybackFailed); ok && failed.Path != "/music/a.flac" {
					t.Errorf("PlaybackFailed.Path = %q, want /music/a.flac", failed.Path)
				}
			}
		})
	}
}

// A client gets the error of its own Play or Toggle back: an event would
// report the failure twice.
func TestPlayback_ClientStartThatFailsToOpenEmitsNothing(t *testing.T) {
	bus := &fakeMPRIS{}
	c, mock := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
	c.Playback().AddTracks(Track{Path: "/music/a.flac"})
	c.Playback().QueueMoveTo(0)
	unreadable := errors.New("unreadable")
	mock.SetPlayError(unreadable)

	if err := c.Playback().Play(); !errors.Is(err, unreadable) {
		t.Errorf("Play = %v, want the open error", err)
	}
	if err := c.Playback().Toggle(); !errors.Is(err, unreadable) {
		t.Errorf("Toggle = %v, want the open error", err)
	}
	expectNoEvent[PlaybackFailed](t, c)
}

// Nor when the start is that of the first track of a queue with no current
// track.
func TestPlayback_ClientStartOfAFirstTrackThatFailsToOpenEmitsNothing(t *testing.T) {
	for name, start := range map[string]func(*Playback) error{
		"Play":   (*Playback).Play,
		"Toggle": (*Playback).Toggle,
	} {
		t.Run(name, func(t *testing.T) {
			bus := &fakeMPRIS{}
			c, mock := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
			c.Playback().AddTracks(Track{Path: "/music/a.flac"})
			unreadable := errors.New("unreadable")
			mock.SetPlayError(unreadable)

			if err := start(c.Playback()); !errors.Is(err, unreadable) {
				t.Errorf("%s = %v, want the open error", name, err)
			}
			expectNoEvent[PlaybackFailed](t, c)
		})
	}
}

// A Next over MPRIS onto a track that fails to open is reported once, as the
// playback service reports it.
func TestMPRIS_NextOntoATrackThatFailsToOpenIsReportedOnce(t *testing.T) {
	bus := &fakeMPRIS{}
	c, mock := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
	c.Playback().AddTracks(Track{Path: "/music/a.flac"}, Track{Path: "/music/b.flac"})
	c.Playback().QueueMoveTo(0)
	if err := bus.served.Play(); err != nil {
		t.Fatalf("play over MPRIS: %v", err)
	}
	mock.SetPlayError(errors.New("unreadable"))

	if err := bus.served.Next(); err == nil {
		t.Fatal("next over MPRIS succeeded, want the open failure")
	}

	if n := count[PlaybackFailed](eventsDuring(c)); n != 1 {
		t.Errorf("%d PlaybackFailed, want 1", n)
	}
}

func TestMPRIS_StartThatSucceedsIsNotAFailedPlay(t *testing.T) {
	bus := &fakeMPRIS{}
	c, _ := newTestCore(t, Options{MPRIS: true, mpris: bus.serve})
	c.Playback().AddTracks(Track{Path: "/music/a.flac"})
	c.Playback().QueueMoveTo(0)

	if err := bus.served.Play(); err != nil {
		t.Fatalf("play over MPRIS: %v", err)
	}
	if err := bus.served.Toggle(); err != nil || !c.Playback().IsPaused() {
		t.Fatalf("toggle while playing: %v, paused %v", err, c.Playback().IsPaused())
	}
	if err := bus.served.Toggle(); err != nil || !c.Playback().IsPlaying() {
		t.Fatalf("toggle while paused: %v, playing %v", err, c.Playback().IsPlaying())
	}
	expectNoEvent[PlaybackFailed](t, c)
}
