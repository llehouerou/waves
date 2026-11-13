package core

import (
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/state"
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
	mgr := linkedState(t)
	addLibraryTracks(t, mgr, "Low", 2)
	addLibraryTracks(t, mgr, "Codeine", 3)

	c, mock := newTestCore(t, Options{
		state:  mgr,
		Lastfm: lastfmConfigured,
		Radio:  (&config.Config{}).GetRadioConfig(),
	})
	c.Playback().AddTracks(
		Track{Path: "/lib/Low/1.mp3", Artist: "Low", Title: "Low 1"},
		Track{Path: "/lib/Low/2.mp3", Artist: "Low", Title: "Low 2"},
	)
	return c, mock
}

func addLibraryTracks(t *testing.T, mgr *state.Manager, artist string, count int) {
	t.Helper()
	for i := 1; i <= count; i++ {
		n := string(rune('0' + i))
		_, err := mgr.DB().Exec(`
			INSERT INTO library_tracks
				(path, mtime, artist, album_artist, album, title, disc_number, track_number, year, added_at, updated_at)
			VALUES (?, 1000, ?, ?, 'Album', ?, 1, ?, 2024, 1000, 1000)
		`, "/lib/"+artist+"/"+n+".mp3", artist, artist, artist+" "+n, i)
		if err != nil {
			t.Fatalf("insert track: %v", err)
		}
	}
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

func TestRadio_FillThatLandsOnAStoppedQueueStartsPlayback(t *testing.T) {
	release := make(chan struct{})
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		<-release
		return codeineIsSimilar(method, params)
	})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock) // or a failure above would leave the fake server waiting
	c, _ := newRadioCore(t)
	radioOn(t, c)

	playAt(t, c, 1)
	waitCall(t, calls, similarMethod)
	if err := c.Playback().Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	unblock()

	waitRadioTracks(t, c)
	eventually(t, "playback starts again", c.Playback().IsPlaying)
}

func TestRadio_FailedFillIsReportedToTheClient(t *testing.T) {
	lastfmtest.Serve(t, func(string, url.Values) string { return "" }) // Last.fm answers every call with an error
	c, _ := newRadioCore(t)
	radioOn(t, c)

	playAt(t, c, 1)

	if e := nextEvent[Failed](t, c); e.Op != errmsg.OpRadioFill || e.Err == nil {
		t.Errorf("Failed = %+v, want a radio fill error", e)
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
		{"Last.fm not linked", lastfmConfigured, false, []RepeatMode{RepeatAll, RepeatOne, RepeatOff}},
		{"no Last.fm", config.LastfmConfig{}, false, []RepeatMode{RepeatAll, RepeatOne, RepeatOff}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := openTestState(t)
			if tt.linked {
				mgr = linkedState(t)
			}
			c, _ := newTestCore(t, Options{state: mgr, Lastfm: tt.lastfm})
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
