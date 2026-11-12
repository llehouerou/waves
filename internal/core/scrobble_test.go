package core

import (
	"net/url"
	"strconv"
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
	c, mock := newTestCore(t, Options{State: linkedState(t), Lastfm: lastfmConfigured})
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
			mgr := openTestState(t)
			if tt.linked {
				mgr = linkedState(t)
			}
			c, mock := newTestCore(t, Options{State: mgr, Lastfm: lastfmConfigured})

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
	c, mock := newTestCore(t, Options{State: linkedState(t), Lastfm: lastfmConfigured})

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

func TestScrobble_RefusedByLastfmIsKeptAsPending(t *testing.T) {
	lastfmtest.Serve(t, func(string, url.Values) string { return "" }) // Last.fm answers every call with an error
	mgr := linkedState(t)
	c, mock := newTestCore(t, Options{State: mgr, Lastfm: lastfmConfigured})

	playTrack(t, c, mock, tags.Tag{Path: "/music/lullaby.flac", Artist: "Low", Title: "Lullaby", Album: "I Could Live in Hope"}, 3*time.Minute)
	mock.SetPosition(2 * time.Minute)

	var pending []state.PendingScrobble
	eventually(t, "the scrobble is pending", func() bool {
		var err error
		pending, err = mgr.GetPendingScrobbles()
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
