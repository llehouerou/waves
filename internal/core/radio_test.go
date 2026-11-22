package core

import (
	"database/sql"
	"net/url"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/player"
)

const similarMethod = "artist.getsimilar"

// codeineIsSimilar answers that Codeine is the artist similar to any other.
func codeineIsSimilar(method string, _ url.Values) string {
	if method == similarMethod {
		return `<lfm status="ok"><similarartists artist="Low">` +
			`<artist><name>Codeine</name><match>0.9</match></artist>` +
			`</similarartists></lfm>`
	}
	return ""
}

// newRadioCore builds a core with Last.fm linked and a library holding two
// tracks by Low and three by Codeine. The queue starts with Low's tracks.
func newRadioCore(t *testing.T) (*Core, *player.Mock) {
	t.Helper()
	return newRadioCoreWith(t, Options{})
}

// newRadioCoreWith is newRadioCore on opts, its database, Last.fm and radio
// set by it.
func newRadioCoreWith(t *testing.T, opts Options) (*Core, *player.Mock) {
	t.Helper()
	seeded := linkedState(t)
	addLibraryTracks(t, seeded.DB(), "Low", 2)
	addLibraryTracks(t, seeded.DB(), "Codeine", 3)

	opts.Database = seeded.closed(t)
	opts.Lastfm = lastfmConfigured
	opts.Radio = (&config.Config{}).GetRadioConfig()
	c, mock := newTestCore(t, opts)
	c.Playback().AddTracks(
		Track{Path: "/lib/Low/1.mp3", Artist: "Low", Title: "Low 1"},
		Track{Path: "/lib/Low/2.mp3", Artist: "Low", Title: "Low 2"},
	)
	return c, mock
}

func addLibraryTracks(t *testing.T, db *sql.DB, artist string, count int) {
	t.Helper()
	for i := 1; i <= count; i++ {
		_, err := db.Exec(`
			INSERT INTO library_tracks
				(path, mtime, artist, album_artist, album, title, disc_number, track_number, year, added_at, updated_at)
			VALUES (?, 1000, ?, ?, 'Album', ?, 1, ?, 2024, 1000, 1000)
		`, libraryPath(artist, i), artist, artist, artist+" "+string(rune('0'+i)), i)
		if err != nil {
			t.Fatalf("insert track: %v", err)
		}
	}
}

// libraryPath is the path of the track number n by artist that
// addLibraryTracks puts in the library.
func libraryPath(artist string, n int) string {
	return "/lib/" + artist + "/" + string(rune('0'+n)) + ".mp3"
}

// radioOn puts the queue in radio mode, the way a client does.
func radioOn(t *testing.T, c *Core) {
	t.Helper()
	for range 3 {
		c.Playback().CycleRepeatMode()
	}
	if mode := c.Playback().RepeatMode(); mode != RepeatRadio {
		t.Fatalf("repeat mode = %v, want Radio", mode)
	}
}

// playAt starts the queue track at index.
func playAt(t *testing.T, c *Core, index int) {
	t.Helper()
	c.Playback().QueueMoveTo(index)
	if err := c.Playback().Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
}

// waitRadioTracks waits until the queue holds more than the two tracks it
// started with, and checks that what radio added is by the similar artist.
func waitRadioTracks(t *testing.T, c *Core) {
	t.Helper()
	pb := c.Playback()
	eventually(t, "radio adds tracks to the queue", func() bool { return pb.QueueLen() > 2 })
	for _, track := range pb.QueueTracks()[2:] {
		if track.Artist != "Codeine" {
			t.Errorf("radio added %q by %q, want a track by Codeine", track.Title, track.Artist)
		}
	}
}

func TestRadio_StartingTheLastTrackFillsTheQueue(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, _ := newRadioCore(t)
	radioOn(t, c)

	playAt(t, c, 0)
	expectNoCall(t, calls, similarMethod) // a track follows: nothing to fetch yet

	if err := c.Playback().Next(); err != nil {
		t.Fatalf("next: %v", err)
	}

	if seed := waitCall(t, calls, similarMethod).Params.Get("artist"); seed != "Low" {
		t.Errorf("seed artist = %q, want Low", seed)
	}
	waitRadioTracks(t, c)
	expectNoCall(t, calls, similarMethod) // one start, one fill
}

func TestRadio_NearTheEndOfTheQueueFillsItWithNoClientTick(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, mock := newRadioCore(t)
	radioOn(t, c)

	// The track that followed is removed while the first one plays: it became
	// the last one without ever starting as the last one.
	playAt(t, c, 0)
	nextEvent[StateChanged](t, c) // the core has seen the track start
	mock.SetDuration(3 * time.Minute)
	mock.SetPosition(time.Minute)
	c.Playback().RemoveTracks([]int{1})
	expectNoCall(t, calls, similarMethod) // two minutes left

	mock.SetPosition(3*time.Minute - 10*time.Second)

	waitCall(t, calls, similarMethod)
	eventually(t, "radio adds tracks to the queue", func() bool { return c.Playback().QueueLen() > 1 })
	expectNoCall(t, calls, similarMethod) // one fill per play
}

func TestRadio_NearTheEndFillsOncePerPlay(t *testing.T) {
	// Last.fm knows no similar artist: the fill adds nothing, and the track is
	// still about to end with nothing after it.
	calls := lastfmtest.Serve(t, func(method string, _ url.Values) string {
		if method == similarMethod {
			return `<lfm status="ok"><similarartists artist="Low"></similarartists></lfm>`
		}
		return ""
	})
	c, mock := newRadioCore(t)
	radioOn(t, c)

	playAt(t, c, 0)
	nextEvent[StateChanged](t, c) // the core has seen the track start
	mock.SetDuration(3 * time.Minute)
	c.Playback().RemoveTracks([]int{1})
	mock.SetPosition(3*time.Minute - 10*time.Second)

	waitCall(t, calls, similarMethod)
	expectNoCall(t, calls, similarMethod)
}

func TestRadio_OffNeverFills(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, mock := newRadioCore(t)

	playAt(t, c, 1)
	mock.SetDuration(3 * time.Minute)
	mock.SetPosition(3*time.Minute - 10*time.Second)

	expectNoCall(t, calls, similarMethod)
}

// heldRadioCore builds a radio core playing the last track of its queue, with
// the fill that track started held back until release is called.
func heldRadioCore(t *testing.T) (c *Core, mock *player.Mock, release func()) {
	t.Helper()
	c, mock, _, release = heldRadioCoreAnswering(t, codeineIsSimilar)
	return c, mock, release
}

// heldRadioCoreAnswering is heldRadioCore on a Last.fm that answers respond
// once released. It returns the calls Last.fm receives.
func heldRadioCoreAnswering(t *testing.T, respond func(string, url.Values) string) (c *Core, mock *player.Mock, calls <-chan lastfmtest.Call, release func()) {
	t.Helper()
	calls, release = heldLastfm(t, respond)
	c, mock = newRadioCore(t)
	radioOn(t, c)

	playAt(t, c, 1)
	waitCall(t, calls, similarMethod)
	settle(c)
	return c, mock, calls, release
}

// settle discards the events of the core until none comes for the quiet
// period: what the test reads next is what it caused.
func settle(c *Core) {
	untilQuiet(c, func(Event) {})
}

// untilQuiet passes the events of the core to seen until none comes for the
// quiet period.
func untilQuiet(c *Core, seen func(Event)) {
	for {
		select {
		case e := <-c.Events():
			seen(e)
		case <-time.After(quiet):
			return
		}
	}
}

// heldLastfm serves a Last.fm that holds every call back until release is
// called, then answers respond.
func heldLastfm(t *testing.T, respond func(string, url.Values) string) (calls <-chan lastfmtest.Call, release func()) {
	t.Helper()
	held := make(chan struct{})
	calls = lastfmtest.Serve(t, func(method string, params url.Values) string {
		<-held
		return respond(method, params)
	})
	release = sync.OnceFunc(func() { close(held) })
	t.Cleanup(release) // or a failure before it would leave the fake server waiting
	return calls, release
}

// tryAgainLater answers every call with an error that fails a fill: Last.fm
// asking to try again later. A refusal (error 6) fails none: radio takes it
// as an answer with nothing (#242).
func tryAgainLater(string, url.Values) string { return lastfmtest.Error(11) }

// failsFirst answers the first artist.getsimilar call with an error that
// fails the fill, and the others as codeineIsSimilar does.
func failsFirst() func(string, url.Values) string {
	var failed atomic.Bool
	return func(method string, params url.Values) string {
		if method == similarMethod && !failed.Swap(true) {
			return tryAgainLater(method, params)
		}
		return codeineIsSimilar(method, params)
	}
}

// nextState waits for the StateChanged that reaches state, skipping the others.
func nextState(t *testing.T, c *Core, state State) {
	t.Helper()
	for nextEvent[StateChanged](t, c).Current != state {
	}
}

// expectResumedOn checks that playback resumed by itself on the queue track
// at index, a track the fill added, and that the client heard of it with no
// Failed; the track that ran out started once.
func expectResumedOn(t *testing.T, c *Core, mock *player.Mock, index int) {
	t.Helper()
	var changed, playing bool
	deadline := time.After(5 * time.Second)
	for !changed || !playing {
		select {
		case e := <-c.Events():
			switch e := e.(type) {
			case TrackChanged:
				changed = changed || e.Index == index
			case StateChanged:
				playing = playing || e.Current == StatePlaying
			case Failed:
				t.Errorf("unexpected Failed: %+v", e)
			}
		case <-deadline:
			t.Fatalf("no resume: TrackChanged to %d seen %v, StateChanged to Playing seen %v", index, changed, playing)
		}
	}

	pb := c.Playback()
	if got := pb.QueueCurrentIndex(); got != index {
		t.Errorf("queue index = %d, want %d", got, index)
	}
	resumed := pb.QueueTracks()[index]
	if resumed.Artist != "Codeine" {
		t.Errorf("resumed on %q by %q, want a track the fill added", resumed.Title, resumed.Artist)
	}
	want := []string{"/lib/Low/2.mp3", resumed.Path}
	if got := mock.PlayCalls(); !slices.Equal(got, want) {
		t.Errorf("player starts = %v, want %v", got, want)
	}
	if !pb.IsPlaying() {
		t.Errorf("state = %v, want Playing", pb.State())
	}
}

// expectNotResumed checks that the fill added its tracks and changed nothing
// else: the player starts are still wantStarts.
func expectNotResumed(t *testing.T, c *Core, mock *player.Mock, wantState State, wantStarts ...string) {
	t.Helper()
	waitRadioTracks(t, c)
	expectNoEvent[StateChanged](t, c)
	expectPlaybackKept(t, c, mock, wantState, wantStarts...)
}

// expectPlaybackKept checks that playback is in wantState on the last of the
// queue's two first tracks, and that the player starts are still wantStarts.
func expectPlaybackKept(t *testing.T, c *Core, mock *player.Mock, wantState State, wantStarts ...string) {
	t.Helper()
	pb := c.Playback()
	if got := pb.State(); got != wantState {
		t.Errorf("state = %v, want %v", got, wantState)
	}
	if got := pb.QueueCurrentIndex(); got != 1 {
		t.Errorf("queue index = %d, want 1", got)
	}
	if got := mock.PlayCalls(); !slices.Equal(got, wantStarts) {
		t.Errorf("player starts = %v, want %v", got, wantStarts)
	}
}

func TestRadio_FillThatLandsAfterAStopStartsNothing(t *testing.T) {
	c, mock, release := heldRadioCore(t)

	if err := c.Playback().Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	nextState(t, c, StateStopped)
	release()

	expectNotResumed(t, c, mock, StateStopped, "/lib/Low/2.mp3")
}

func TestRadio_FillThatLandsAfterTheLastTrackFinishedResumesOnTheNextTrack(t *testing.T) {
	c, mock, release := heldRadioCore(t)

	mock.SimulateFinished()
	nextState(t, c, StateStopped)
	release()

	expectResumedOn(t, c, mock, 2)
}

func TestRadio_FillThatLandsAfterANextPastTheEndResumesOnTheNextTrack(t *testing.T) {
	c, mock, release := heldRadioCore(t)

	if err := c.Playback().Next(); err != nil {
		t.Fatalf("next: %v", err)
	}
	nextState(t, c, StateStopped)
	release()

	expectResumedOn(t, c, mock, 2)
}

func TestRadio_StopAfterTheQueueRanOutCancelsTheResume(t *testing.T) {
	c, mock, release := heldRadioCore(t)

	mock.SimulateFinished()
	nextState(t, c, StateStopped)
	if err := c.Playback().Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	release()

	expectNotResumed(t, c, mock, StateStopped, "/lib/Low/2.mp3")
}

// expectDropped checks that the fill that landed changed nothing: no
// QueueChanged, StateChanged or Failed reaches the client, the queue still
// holds its two tracks on the last one, radio remembers none of the similar
// artist's tracks, and the player starts are still wantStarts.
func expectDropped(t *testing.T, c *Core, mock *player.Mock, wantState State, wantStarts ...string) {
	t.Helper()
	untilQuiet(c, func(e Event) {
		switch e.(type) {
		case QueueChanged, StateChanged, Failed:
			t.Errorf("unexpected %T: %+v", e, e)
		}
	})

	if got := c.Playback().QueueLen(); got != 2 {
		t.Errorf("queue holds %d tracks, want 2", got)
	}
	for i := 1; i <= 3; i++ {
		if path := libraryPath("Codeine", i); c.radio.IsRecentlyPlayed(path) {
			t.Errorf("%s is recently played, want the dropped fill forgotten", path)
		}
	}
	expectPlaybackKept(t, c, mock, wantState, wantStarts...)
}

func TestRadio_FillThatLandsAfterRadioOffIsDropped(t *testing.T) {
	c, mock, release := heldRadioCore(t)

	if mode := c.Playback().CycleRepeatMode(); mode != RepeatOff {
		t.Fatalf("repeat mode = %v, want Off", mode)
	}
	release()

	expectDropped(t, c, mock, StatePlaying, "/lib/Low/2.mp3")
}

func TestRadio_FillThatLandsAfterRadioLeftOverTheServiceIsDropped(t *testing.T) {
	c, mock, release := heldRadioCore(t)

	c.playback.svc.SetRepeatMode(RepeatAll) // how MPRIS LoopStatus leaves radio mode
	release()

	expectDropped(t, c, mock, StatePlaying, "/lib/Low/2.mp3")
}

func TestRadio_FailedFillThatLandsAfterRadioOffIsNotReported(t *testing.T) {
	c, mock, _, release := heldRadioCoreAnswering(t, tryAgainLater)

	c.Playback().CycleRepeatMode() // radio -> off
	release()

	expectDropped(t, c, mock, StatePlaying, "/lib/Low/2.mp3")
}

func TestRadio_RadioOffAfterTheQueueRanOutCancelsTheResume(t *testing.T) {
	c, mock, release := heldRadioCore(t)

	mock.SimulateFinished()
	nextState(t, c, StateStopped)
	c.Playback().CycleRepeatMode() // radio -> off
	release()

	expectDropped(t, c, mock, StateStopped, "/lib/Low/2.mp3")
}

func TestRadio_RadioOnAgainAfterADroppedFillFillsOnTheNextStart(t *testing.T) {
	c, mock, calls, release := heldRadioCoreAnswering(t, failsFirst())

	c.Playback().CycleRepeatMode() // radio -> off
	release()
	expectDropped(t, c, mock, StatePlaying, "/lib/Low/2.mp3")

	radioOn(t, c)
	playAt(t, c, 1)

	waitCall(t, calls, similarMethod)
	waitRadioTracks(t, c)
}

func TestRadio_RadioOnAgainAfterADroppedNearEndFillFillsTheSamePlay(t *testing.T) {
	calls, release := heldLastfm(t, failsFirst())
	c, mock := newRadioCore(t)
	radioOn(t, c)

	// The first track becomes the last one while it plays, and nears its end.
	playAt(t, c, 0)
	nextEvent[StateChanged](t, c) // the core has seen the track start
	mock.SetDuration(3 * time.Minute)
	c.Playback().RemoveTracks([]int{1})
	mock.SetPosition(3*time.Minute - 10*time.Second)
	waitCall(t, calls, similarMethod)

	c.Playback().CycleRepeatMode() // radio -> off
	release()
	expectNoEvent[Failed](t, c) // the fill landed, dropped

	radioOn(t, c)

	waitCall(t, calls, similarMethod)
	eventually(t, "radio adds tracks to the queue", func() bool { return c.Playback().QueueLen() > 1 })
}

func TestRadio_FillThatLandsAfterRadioOffAndOnAgainIsAdded(t *testing.T) {
	c, _, release := heldRadioCore(t)

	for range 4 { // radio -> off -> all -> one -> radio
		c.Playback().CycleRepeatMode()
	}
	if mode := c.Playback().RepeatMode(); mode != RepeatRadio {
		t.Fatalf("repeat mode = %v, want Radio", mode)
	}
	release()

	waitRadioTracks(t, c)
}

func TestRadio_PlayAfterTheQueueRanOutReplaysTheLastTrackAndTheFillOnlyAddsTracks(t *testing.T) {
	c, mock, release := heldRadioCore(t)

	mock.SimulateFinished()
	nextState(t, c, StateStopped)
	if err := c.Playback().Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	nextState(t, c, StatePlaying)
	release()

	expectNotResumed(t, c, mock, StatePlaying, "/lib/Low/2.mp3", "/lib/Low/2.mp3")
}

func TestRadio_FillThatLandsOnAQueueClearedBeforeItRanOutPlaysItsFirstTrack(t *testing.T) {
	c, mock, release := heldRadioCore(t)

	c.Playback().ClearQueue()
	mock.SimulateFinished()
	nextState(t, c, StateStopped)
	release()

	expectResumedOn(t, c, mock, 0)
	expectNoEvent[Failed](t, c)
}

func TestRadio_FailedFillIsReportedToTheClient(t *testing.T) {
	lastfmtest.Serve(t, tryAgainLater)
	c, _ := newRadioCore(t)
	radioOn(t, c)

	playAt(t, c, 1)

	if e := nextEvent[Failed](t, c); e.Op != errmsg.OpRadioFill || e.Err == nil {
		t.Errorf("Failed = %+v, want a radio fill error", e)
	}
}

// Issue #242: a seed Last.fm refuses is a seed with no similar artist.
func TestRadio_SeedLastfmRefusesFillsFromTheTracksRadioPlayedWithNoFailed(t *testing.T) {
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		if method != similarMethod {
			return ""
		}
		switch params.Get("artist") {
		case "Low":
			return codeineIsSimilar(method, params)
		case "Codeine":
			return `<lfm status="ok"><similarartists artist="Codeine">` +
				`<artist><name>Low</name><match>0.9</match></artist>` +
				`</similarartists></lfm>`
		}
		return lastfmtest.Error(6)
	})
	c, _ := newRadioCore(t)
	radioOn(t, c)
	pb := c.Playback()

	// A first fill from Low puts tracks by Codeine in what radio played.
	playAt(t, c, 1)
	waitCall(t, calls, similarMethod)
	waitRadioTracks(t, c)
	pb.AddTracks(Track{Path: "/lib/Galaxie 500/1.mp3", Artist: "Galaxie 500", Title: "Tugboat"})
	queued := pb.QueueLen()
	settle(c)

	playAt(t, c, queued-1) // Last.fm refuses Galaxie 500: Codeine is the fallback seed

	if seed := waitCall(t, calls, similarMethod).Params.Get("artist"); seed != "Galaxie 500" {
		t.Fatalf("seed artist = %q, want Galaxie 500", seed)
	}
	deadline := time.After(5 * time.Second)
	for pb.QueueLen() == queued {
		select {
		case e := <-c.Events():
			if failed, ok := e.(Failed); ok {
				t.Fatalf("Failed = %+v, want the fill to go on from Codeine", failed)
			}
		case <-deadline:
			t.Fatal("radio added no track")
		}
	}
	for _, track := range pb.QueueTracks()[queued:] {
		if track.Artist != "Low" {
			t.Errorf("radio added %q by %q, want a track by Low", track.Title, track.Artist)
		}
	}
}

func TestCycleRepeatMode(t *testing.T) {
	tests := []struct {
		name   string
		lastfm config.LastfmConfig
		linked bool
		want   []RepeatMode
	}{
		{"Last.fm linked", lastfmConfigured, true, []RepeatMode{RepeatAll, RepeatOne, RepeatRadio, RepeatOff}},
		// Radio needs Last.fm configured, not an account (#197).
		{"Last.fm not linked", lastfmConfigured, false, []RepeatMode{RepeatAll, RepeatOne, RepeatRadio, RepeatOff}},
		{"no Last.fm", config.LastfmConfig{}, false, []RepeatMode{RepeatAll, RepeatOne, RepeatOff}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts Options
			if tt.linked {
				opts.Database = linkedState(t).closed(t)
			}
			opts.Lastfm = tt.lastfm
			c, _ := newTestCore(t, opts)
			for i, want := range tt.want {
				if got := c.Playback().CycleRepeatMode(); got != want {
					t.Errorf("cycle %d = %v, want %v", i+1, got, want)
				}
				if got := c.Playback().RepeatMode(); got != want {
					t.Errorf("mode after cycle %d = %v, want %v", i+1, got, want)
				}
			}
		})
	}
}
