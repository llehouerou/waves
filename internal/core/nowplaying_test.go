package core

import (
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
	"github.com/llehouerou/waves/internal/player"
	"github.com/llehouerou/waves/internal/tags"
)

const nowPlayingMethod = "track.updatenowplaying"

// acceptNowPlaying answers every "now playing" update and every scrobble as
// accepted.
func acceptNowPlaying(method string, params url.Values) string {
	if method == nowPlayingMethod {
		return `<lfm status="ok"><nowplaying></nowplaying></lfm>`
	}
	return acceptScrobbles(method, params)
}

// startTrack queues the track of a file holding tag and starts it, the way a
// client would. The mock player reads the file's tags when it opens it, as
// the real one does: they are there when the track starts.
func startTrack(t *testing.T, c *Core, mock *player.Mock, tag tags.Tag, duration time.Duration) {
	t.Helper()
	mock.SetFileInfo(&tags.FileInfo{Tag: tag})
	mock.SetDuration(duration)
	pb := c.Playback()
	pb.AddTracks(Track{Path: tag.Path, Artist: "what the queue shows", Title: "what the queue shows"})
	pb.QueueMoveTo(pb.QueueLen() - 1)
	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
}

// Issue #252: Last.fm was never told what plays.
func TestNowPlaying_ATrackStartTellsLastfmWhatTheFileSays(t *testing.T) {
	tests := []struct {
		name            string
		albumArtist     string
		wantAlbumArtist string
	}{
		{"album artist of its own", "Various Artists", "Various Artists"},
		{"album artist the same as the artist", "Low", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := lastfmtest.Serve(t, acceptNowPlaying)
			c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})

			startTrack(t, c, mock, tags.Tag{
				Path:          "/music/lullaby.flac",
				Artist:        "Low",
				Title:         "Lullaby",
				Album:         "I Could Live in Hope",
				AlbumArtist:   tt.albumArtist,
				MBRecordingID: "5b0f1ff8-6e3f-4a6c-8c0e-2b6f6c0e6a10",
			}, 3*time.Minute)

			// At 0:00 of 3:00: long before the scrobble threshold.
			call := waitCall(t, calls, nowPlayingMethod)
			want := map[string]string{
				"artist":      "Low",
				"track":       "Lullaby",
				"album":       "I Could Live in Hope",
				"albumArtist": tt.wantAlbumArtist,
				"duration":    "180",
				"mbid":        "5b0f1ff8-6e3f-4a6c-8c0e-2b6f6c0e6a10",
				"timestamp":   "",
			}
			for param, value := range want {
				if got := call.Params.Get(param); got != value {
					t.Errorf("%s = %q, want %q", param, got, value)
				}
			}
			expectNoCall(t, calls, nowPlayingMethod) // once per track start
		})
	}
}

// Each start of a track sends an update, whatever started it; a resume goes
// on with the same play and sends none.
func TestNowPlaying_EachStartSendsAnUpdateAndAResumeDoesNot(t *testing.T) {
	lullaby := tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}
	tests := []struct {
		name    string
		restart func(c *Core, mock *player.Mock) error
	}{
		{"Play on the track playing", func(c *Core, _ *player.Mock) error { return c.Playback().Play() }},
		{"waves restarts it by repeat-one", func(c *Core, mock *player.Mock) error {
			c.Playback().SetRepeatMode(RepeatOne)
			mock.SimulateFinished()
			return nil
		}},
		{"the player restarts it by repeat-one", func(c *Core, mock *player.Mock) error {
			c.Playback().SetRepeatMode(RepeatOne)
			mock.SimulateGaplessSwitch(lullaby.Path)
			return nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := lastfmtest.Serve(t, acceptNowPlaying)
			c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})
			startTrack(t, c, mock, lullaby, 3*time.Minute)
			waitCall(t, calls, nowPlayingMethod)

			if err := c.Playback().Pause(); err != nil {
				t.Fatalf("pause: %v", err)
			}
			if err := c.Playback().Toggle(); err != nil {
				t.Fatalf("resume: %v", err)
			}
			expectNoCall(t, calls, nowPlayingMethod)

			if err := tt.restart(c, mock); err != nil {
				t.Fatalf("restart: %v", err)
			}
			if got := waitCall(t, calls, nowPlayingMethod).Params.Get("track"); got != "Lullaby" {
				t.Errorf("second update = %q, want Lullaby", got)
			}
			expectNoCall(t, calls, nowPlayingMethod)
		})
	}
}

// A track too short to be scrobbled plays all the same: its start sends an
// update.
func TestNowPlaying_ATrackTooShortToBeScrobbledSendsAnUpdate(t *testing.T) {
	calls := lastfmtest.Serve(t, acceptNowPlaying)
	c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})

	startTrack(t, c, mock, tags.Tag{Path: "/music/intro.flac", Artist: "Low", Title: "Intro"}, 20*time.Second)

	if got := waitCall(t, calls, nowPlayingMethod).Params.Get("track"); got != "Intro" {
		t.Errorf("update = %q, want Intro", got)
	}
}

func TestNowPlaying_NoUpdateIsSentWithNoAccountLinked(t *testing.T) {
	tests := []struct {
		name   string
		config func(t *testing.T) Options
		before func(t *testing.T, c *Core)
	}{
		{"no session saved", func(*testing.T) Options { return Options{Lastfm: lastfmConfigured} }, nil},
		{"an API key and no secret", func(t *testing.T) Options {
			return Options{Database: linkedState(t).closed(t), Lastfm: config.LastfmConfig{APIKey: "key"}}
		}, nil},
		{"after Unlink", func(t *testing.T) Options {
			return Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured}
		}, func(t *testing.T, c *Core) {
			if err := c.Lastfm().Unlink(); err != nil {
				t.Fatalf("unlink: %v", err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := lastfmtest.Serve(t, acceptNowPlaying)
			c, mock := newTestCore(t, tt.config(t))
			if tt.before != nil {
				tt.before(t, c)
			}

			startTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)

			expectNoCall(t, calls, nowPlayingMethod)
		})
	}
}

// An account linked in the middle of a track is told of the next one.
func TestNowPlaying_AnAccountLinkedMidTrackHearsOfTheNextStart(t *testing.T) {
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		if method == nowPlayingMethod {
			return acceptNowPlaying(method, params)
		}
		return lastfmLinks(method, params)
	})
	c, mock := newTestCore(t, Options{Lastfm: lastfmConfigured})
	startTrack(t, c, mock, tags.Tag{Path: "/music/1.flac", Artist: "Low", Title: "First"}, 3*time.Minute)

	link(t, c)
	expectNoCall(t, calls, nowPlayingMethod)

	startTrack(t, c, mock, tags.Tag{Path: "/music/2.flac", Artist: "Low", Title: "Second"}, 3*time.Minute)
	if got := waitCall(t, calls, nowPlayingMethod).Params.Get("track"); got != "Second" {
		t.Errorf("update = %q, want Second", got)
	}
}

// The update is best effort: whatever Last.fm makes of it, the user hears
// nothing, the account stays linked, and the scrobble of the play goes out.
func TestNowPlaying_AFailedUpdateCostsNothing(t *testing.T) {
	tests := []struct{ name, answer string }{
		{"Last.fm refuses it", lastfmtest.Error(6)},
		{"Last.fm cannot be reached", lastfmtest.Unreachable},
		{"Last.fm no longer accepts the session", lastfmtest.Error(9)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
				if method == nowPlayingMethod {
					return tt.answer
				}
				return acceptScrobbles(method, params)
			})
			c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})

			startTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
			waitCall(t, calls, nowPlayingMethod)

			expectNoEvent[Failed](t, c)
			if username, linked := c.Lastfm().Account(); !linked || username != "lowfan" {
				t.Errorf("account = %q, linked %v, want lowfan still linked", username, linked)
			}
			if pending := pendingScrobbles(t, c.state); len(pending) != 0 {
				t.Errorf("pending = %+v, want none", pending)
			}
			expectNoCall(t, calls, nowPlayingMethod) // not sent again

			mock.SetPosition(2 * time.Minute)
			if got := waitCall(t, calls, scrobbleMethod).Params.Get("track[0]"); got != "Lullaby" {
				t.Errorf("scrobble = %q, want Lullaby", got)
			}
		})
	}
}

// A Last.fm that does not answer the update holds neither the core's loop
// nor Close.
func TestNowPlaying_AnUpdateLastfmDoesNotAnswerHoldsNothing(t *testing.T) {
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		if method == nowPlayingMethod {
			return lastfmtest.Silent
		}
		return acceptScrobbles(method, params)
	})
	c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})
	startTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby"}, 3*time.Minute)
	waitCall(t, calls, nowPlayingMethod)

	if err := c.Playback().Pause(); err != nil {
		t.Fatalf("pause: %v", err)
	}
	for nextEvent[StateChanged](t, c).Current != StatePaused {
		// the events of the loop still reach the client
	}
	if err := c.Playback().Toggle(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	mock.SetPosition(2 * time.Minute)
	waitCall(t, calls, scrobbleMethod) // the loop still checks the scrobble

	select {
	case <-closeInBackground(c):
	case <-time.After(5 * time.Second):
		t.Fatal("Close waits for the update Last.fm does not answer")
	}
	expectNoFailureAfterClose(t, c)
}

// Tracks that start while an update is in flight: Last.fm hears of the last
// one only, once the one in flight is answered.
func TestNowPlaying_TheUpdateOfTheLastTrackStartedWhileOneIsInFlightIsSentAfterIt(t *testing.T) {
	answered := make(chan struct{})
	calls := lastfmtest.Serve(t, func(method string, params url.Values) string {
		if method == nowPlayingMethod && params.Get("track") == "First" {
			<-answered
		}
		return acceptNowPlaying(method, params)
	})
	c, mock := newTestCore(t, Options{Database: linkedState(t).closed(t), Lastfm: lastfmConfigured})
	answer := sync.OnceFunc(func() { close(answered) }) // Last.fm answers First at last
	t.Cleanup(answer)                                   // before the core closes: a failed test must not leave the call held

	titles := []string{"First", "Second", "Third", "Fourth"}
	for _, title := range titles {
		mock.SetFileInfo(&tags.FileInfo{Tag: tags.Tag{Path: "/music/" + title + ".flac", Artist: "Low", Title: title}})
		c.Playback().AddTracks(Track{Path: "/music/" + title + ".flac"})
	}
	mock.SetDuration(3 * time.Minute)
	c.Playback().QueueMoveTo(0)
	if err := c.Playback().Play(); err != nil {
		t.Fatalf("play: %v", err)
	}
	waitCall(t, calls, nowPlayingMethod) // First, held by Last.fm

	for i := range titles[1:] {
		if err := c.Playback().Next(); err != nil {
			t.Fatalf("next: %v", err)
		}
		for nextEvent[TrackChanged](t, c).Index != i+1 {
			// skip to the change to this track: the loop has read its start,
			// while it plays
		}
	}
	expectNoCall(t, calls, nowPlayingMethod)

	answer()
	if got := waitCall(t, calls, nowPlayingMethod).Params.Get("track"); got != "Fourth" {
		t.Errorf("update after the one in flight = %q, want Fourth", got)
	}
	expectNoCall(t, calls, nowPlayingMethod)
}
