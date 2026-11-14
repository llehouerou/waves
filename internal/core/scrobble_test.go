package core

import (
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/tags"
)

const scrobbleMethod = "track.scrobble"

// acceptScrobbles answers every scrobble as accepted.
func acceptScrobbles(method string, _ url.Values) string {
	if method == scrobbleMethod {
		return `<lfm status="ok"><scrobbles accepted="1" ignored="0"></scrobbles></lfm>`
	}
	return ""
}

// playTrack queues one track and starts it, the way a client would, then gives
// the mock player what the real one reads from the file.
func playTrack(t *testing.T, c *Core, mock *player.Mock, tag tags.Tag, duration time.Duration) {
	t.Helper()
	pb := c.Playback()
	pb.AddTracks(Track{Path: tag.Path, Artist: tag.Artist, Title: tag.Title, Album: tag.Album})
	pb.QueueMoveTo(pb.QueueLen() - 1)
	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	mock.SetTrackInfo(&tags.FileInfo{Tag: tag})
	mock.SetDuration(duration)
}

func TestScrobble_TrackPlayedPastItsThresholdIsScrobbledWithNoClientTick(t *testing.T) {
	calls := lastfmtest.Serve(t, acceptScrobbles)
	c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})
	started := time.Now()

	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby", Album: "I Could Live in Hope"}, 3*time.Minute)
	expectNoCall(t, calls, scrobbleMethod) // at 0:00 of 3:00, nothing to scrobble yet

	mock.SetPosition(91 * time.Second) // past half of the track

	call := waitCall(t, calls, scrobbleMethod)
	if got := call.Params.Get("artist[0]"); got != "Low" {
		t.Errorf("artist = %q, want Low", got)
	}
	if got := call.Params.Get("track[0]"); got != "Lullaby" {
		t.Errorf("track = %q, want Lullaby", got)
	}
	if got := call.Params.Get("album[0]"); got != "I Could Live in Hope" {
		t.Errorf("album = %q, want I Could Live in Hope", got)
	}
	if got := call.Params.Get("duration[0]"); got != "180" {
		t.Errorf("duration = %q, want 180", got)
	}
	// The scrobble is dated at the start of the play, not at the threshold.
	ts, err := strconv.ParseInt(call.Params.Get("timestamp[0]"), 10, 64)
	if err != nil {
		t.Fatalf("timestamp %q: %v", call.Params.Get("timestamp[0]"), err)
	}
	if at := time.Unix(ts, 0); at.Before(started.Add(-time.Second)) || at.After(time.Now()) {
		t.Errorf("timestamp = %v, want the start of the play (%v)", at, started)
	}

	expectNoCall(t, calls, scrobbleMethod) // once per play
}

func TestScrobble_Threshold(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		position time.Duration
		linked   bool
		want     bool
	}{
		{"just under half of the track", 3 * time.Minute, 89 * time.Second, true, false},
		{"four minutes of a long track", 20 * time.Minute, 4*time.Minute + time.Second, true, true},
		{"under four minutes of a long track", 20 * time.Minute, 3*time.Minute + 59*time.Second, true, false},
		{"a track shorter than 30 seconds, played to the end", 29 * time.Second, 29 * time.Second, true, false},
		{"Last.fm not linked", 3 * time.Minute, 2 * time.Minute, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := lastfmtest.Serve(t, acceptScrobbles)
			opts := Options{Lastfm: lastfmConfigured}
			if tt.linked {
				opts.Database = linkedState(t).closed(t)
			}
			c, mock := newTestCore(t, opts)

			playTrack(t, c, mock, tags.Tag{Path: "/music/a.flac", Artist: "A", Title: "T"}, tt.duration)
			mock.SetPosition(tt.position)

			if tt.want {
				waitCall(t, calls, scrobbleMethod)
			} else {
				expectNoCall(t, calls, scrobbleMethod)
			}
		})
	}
}

func TestScrobble_EachTrackPlayedIsScrobbled(t *testing.T) {
	calls := lastfmtest.Serve(t, acceptScrobbles)
	c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})

	playTrack(t, c, mock, tags.Tag{Path: "/music/1.flac", Artist: "A", Title: "First"}, 3*time.Minute)
	mock.SetPosition(2 * time.Minute)
	if got := waitCall(t, calls, scrobbleMethod).Params.Get("track[0]"); got != "First" {
		t.Fatalf("first scrobble = %q, want First", got)
	}

	// The next track starts from the beginning: a new play, a new scrobble.
	mock.SetPosition(0)
	c.Playback().AddTracks(Track{Path: "/music/2.flac"})
	if err := c.Playback().Next(); err != nil {
		t.Fatalf("next: %v", err)
	}
	mock.SetTrackInfo(&tags.FileInfo{Tag: tags.Tag{Path: "/music/2.flac", Artist: "A", Title: "Second"}})
	expectNoCall(t, calls, scrobbleMethod)

	mock.SetPosition(2 * time.Minute)
	if got := waitCall(t, calls, scrobbleMethod).Params.Get("track[0]"); got != "Second" {
		t.Errorf("second scrobble = %q, want Second", got)
	}
}

// scrobbleTime is the timestamp a scrobble is dated at.
func scrobbleTime(t *testing.T, call lastfmtest.Call) time.Time {
	t.Helper()
	ts, err := strconv.ParseInt(call.Params.Get("timestamp[0]"), 10, 64)
	if err != nil {
		t.Fatalf("timestamp %q: %v", call.Params.Get("timestamp[0]"), err)
	}
	return time.Unix(ts, 0)
}

// Repeat-one: the track that starts again is a new play, scrobbled again and
// dated at its second start, whether waves or the player restarted it.
func TestScrobble_RepeatOneScrobblesEachPlay(t *testing.T) {
	lullaby := tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}
	tests := []struct {
		name    string
		restart func(mock *player.Mock)
	}{
		{"waves restarts it", func(mock *player.Mock) { mock.SimulateFinished() }},
		{"the player restarts it", func(mock *player.Mock) { mock.SimulateGaplessSwitch(lullaby.Path) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := lastfmtest.Serve(t, acceptScrobbles)
			c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})
			c.Playback().SetRepeatMode(RepeatOne)

			playTrack(t, c, mock, lullaby, 3*time.Minute)
			mock.SetPosition(2 * time.Minute)
			waitCall(t, calls, scrobbleMethod)

			time.Sleep(time.Second) // a timestamp counts seconds: the second start is a later one
			restarted := time.Now().Truncate(time.Second)
			mock.SetPosition(0)
			tt.restart(mock)
			eventually(t, "the track starts again", func() bool { return mock.TrackStarts() == 2 })
			mock.SetTrackInfo(&tags.FileInfo{Tag: lullaby})
			expectNoCall(t, calls, scrobbleMethod)

			mock.SetPosition(2 * time.Minute)
			call := waitCall(t, calls, scrobbleMethod)
			if got := call.Params.Get("track[0]"); got != "Lullaby" {
				t.Errorf("second scrobble = %q, want Lullaby", got)
			}
			if at := scrobbleTime(t, call); at.Before(restarted) {
				t.Errorf("second scrobble dated %v, want its second start (%v)", at, restarted)
			}
		})
	}
}

// Resuming from pause goes on with the same play: it is not scrobbled again.
func TestScrobble_PauseThenResumeIsOnePlay(t *testing.T) {
	calls := lastfmtest.Serve(t, acceptScrobbles)
	c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})

	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
	mock.SetPosition(2 * time.Minute)
	waitCall(t, calls, scrobbleMethod)

	if err := c.Playback().Pause(); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := c.Playback().Toggle(); err != nil {
		t.Fatalf("resume: %v", err)
	}

	expectNoCall(t, calls, scrobbleMethod)
}

// Next from pause (a media key, over MPRIS) starts the next track: it is a play
// like any other, scrobbled once past its threshold (issue #123).
func TestScrobble_NextFromPauseIsScrobbled(t *testing.T) {
	calls := lastfmtest.Serve(t, acceptScrobbles)
	c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})

	playTrack(t, c, mock, tags.Tag{Path: "/music/1.flac", Artist: "A", Title: "First"}, 3*time.Minute)
	c.Playback().AddTracks(Track{Path: "/music/2.flac"})
	if err := c.Playback().Pause(); err != nil {
		t.Fatalf("pause: %v", err)
	}
	for nextEvent[StateChanged](t, c).Current != StatePaused {
		// skip to the pause: then the loop has seen it and stopped its clock
	}

	if err := c.Playback().Next(); err != nil {
		t.Fatalf("next: %v", err)
	}
	mock.SetTrackInfo(&tags.FileInfo{Tag: tags.Tag{Path: "/music/2.flac", Artist: "A", Title: "Second"}})
	mock.SetPosition(2 * time.Minute)

	if got := waitCall(t, calls, scrobbleMethod).Params.Get("track[0]"); got != "Second" {
		t.Errorf("scrobble = %q, want Second", got)
	}
}

func TestScrobble_RefusedByLastfmIsKeptAsPending(t *testing.T) {
	lastfmtest.Serve(t, func(string, url.Values) string { return "" }) // Last.fm answers every call with an error
	c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})

	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby", Album: "I Could Live in Hope"}, 3*time.Minute)
	mock.SetPosition(2 * time.Minute)

	var pending []state.PendingScrobble
	eventually(t, "the scrobble is pending", func() bool {
		var err error
		pending, err = c.state.GetPendingScrobbles()
		if err != nil {
			t.Fatalf("pending scrobbles: %v", err)
		}
		return len(pending) > 0
	})
	if len(pending) != 1 {
		t.Fatalf("pending = %d scrobbles, want 1", len(pending))
	}
	if p := pending[0]; p.Artist != "Low" || p.Track != "Lullaby" || p.Album != "I Could Live in Hope" || p.DurationSecs != 180 {
		t.Errorf("pending = %+v, want Low - Lullaby, 180s", p)
	}
}

// playPastThreshold plays Low - Lullaby, three minutes long, past half of it:
// the core scrobbles it on its next tick.
func playPastThreshold(t *testing.T, c *Core, mock *player.Mock) {
	t.Helper()
	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby", Album: "I Could Live in Hope"}, 3*time.Minute)
	mock.SetPosition(2 * time.Minute)
}

// closeInBackground closes the core off the test's goroutine, and returns
// what is closed once Close returned.
func closeInBackground(c *Core) <-chan struct{} {
	closed := make(chan struct{})
	go func() {
		c.Close()
		close(closed)
	}()
	return closed
}

// expectNoFailureAfterClose fails the test if the event stream of a closed core carries
// a failure.
func expectNoFailureAfterClose(t *testing.T, c *Core) {
	t.Helper()
	for e := range c.Events() {
		if failed, ok := e.(Failed); ok {
			t.Errorf("Close reported %+v", failed)
		}
	}
}

// Issue #140: the scrobble of the track playing was neither sent nor kept as
// pending when Close cut it short.
func TestClose_WaitsForTheScrobbleOfTheTrackPlaying(t *testing.T) {
	tests := []struct {
		name        string
		answer      func(method string, params url.Values) string
		wantPending int
	}{
		{"Last.fm takes it", acceptScrobbles, 0},
		{"Last.fm refuses it", func(string, url.Values) string { return lastfmtest.Error(6) }, 1},
		{"Last.fm cannot be reached", func(string, url.Values) string { return lastfmtest.Unreachable }, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answered := make(chan struct{})
			answer := sync.OnceFunc(func() { close(answered) }) // Last.fm answers at last
			calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
				<-answered
				return tt.answer(method, params)
			})
			database := linkedState(t).closed(t)
			// Last.fm answers well within the three ticks Close waits for it.
			c, mock := newTestCore(t, Options{Database: database, Lastfm: lastfmConfigured, tick: 500 * time.Millisecond})
			t.Cleanup(answer) // before the core closes: a failed test must not leave it waiting for Last.fm
			playPastThreshold(t, c, mock)
			waitCall(t, calls, scrobbleMethod)

			closed := closeInBackground(c)
			select {
			case <-closed:
				t.Fatal("Close returned while the scrobble of the track playing was being sent")
			case <-time.After(quiet):
			}

			answer()
			select {
			case <-closed:
			case <-time.After(5 * time.Second):
				t.Fatal("Close still waits after Last.fm answered")
			}
			expectNoFailureAfterClose(t, c)
			if pending := pendingScrobbles(t, reopen(t, database)); len(pending) != tt.wantPending {
				t.Errorf("pending = %+v, want %d", pending, tt.wantPending)
			}
		})
	}
}

// Quitting does not wait for a Last.fm that does not answer: the play is kept
// as a pending scrobble, and the next run sends it.
func TestClose_KeepsTheScrobbleOfTheTrackPlayingAsPendingAfterThreeTicks(t *testing.T) {
	answered := make(chan struct{})
	answer := sync.OnceFunc(func() { close(answered) })
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		<-answered // not before the test ends
		return acceptScrobbles(method, params)
	})
	database := linkedState(t).closed(t)
	const tick = 50 * time.Millisecond
	c, mock := newTestCore(t, Options{Database: database, Lastfm: lastfmConfigured, tick: tick})
	t.Cleanup(answer) // before the core closes: a failed test must not leave it waiting for Last.fm
	started := time.Now()
	playPastThreshold(t, c, mock)
	waitCall(t, calls, scrobbleMethod)

	began := time.Now()
	closed := closeInBackground(c)
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close waits for a Last.fm that does not answer")
	}
	if waited := time.Since(began); waited < 3*tick {
		t.Errorf("Close returned after %v, want it to wait three ticks (%v) for Last.fm", waited, 3*tick)
	}
	expectNoFailureAfterClose(t, c)
	pending := pendingScrobbles(t, reopen(t, database))
	if len(pending) != 1 {
		t.Fatalf("pending = %+v, want the play kept for the next run", pending)
	}
	p := pending[0]
	if p.Artist != "Low" || p.Track != "Lullaby" || p.Album != "I Could Live in Hope" || p.DurationSecs != 180 || p.Attempts != 0 {
		t.Errorf("pending = %+v, want Low - Lullaby on I Could Live in Hope, 180s, no attempt counted", p)
	}
	// Dated at the start of the play, to the second Last.fm keeps.
	if p.Timestamp.Before(started.Add(-time.Second)) || p.Timestamp.After(began) {
		t.Errorf("timestamp = %v, want the start of the play (%v)", p.Timestamp, started)
	}
}

// A pass over the pending scrobbles and the scrobble of the track playing,
// both in flight at quit, share the three ticks: Close waits three, not six.
func TestClose_WaitsThreeTicksInAllForBothScrobblesBeingSent(t *testing.T) {
	answered := make(chan struct{})
	answer := sync.OnceFunc(func() { close(answered) })
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		<-answered // not before the test ends
		return acceptScrobbles(method, params)
	})
	seeded := linkedState(t)
	addPending(t, seeded.Manager, "Words")
	database := seeded.closed(t)
	const tick = 200 * time.Millisecond
	c, mock := newTestCore(t, Options{Database: database, Lastfm: lastfmConfigured, tick: tick})
	t.Cleanup(answer)                  // before the core closes: a failed test must not leave it waiting for Last.fm
	waitCall(t, calls, scrobbleMethod) // the pass sends Words
	playPastThreshold(t, c, mock)
	waitCall(t, calls, scrobbleMethod) // the play of Lullaby

	began := time.Now()
	closed := closeInBackground(c)
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close waits for a Last.fm that does not answer")
	}
	if waited := time.Since(began); waited < 3*tick || waited >= 5*tick {
		t.Errorf("Close returned after %v, want three ticks (%v) in all", waited, 3*tick)
	}
	expectNoFailureAfterClose(t, c)
	if pending := pendingScrobbles(t, reopen(t, database)); len(pending) != 2 {
		t.Errorf("pending = %+v, want Words and Lullaby left for the next run", pending)
	}
}
