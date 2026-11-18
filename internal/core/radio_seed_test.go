package core

import (
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/lastfm/lastfmtest"
)

// Issue #339: a fill is seeded on the artist of the current track of the
// queue when it starts. Radio stores no seed between fills.
//
// Each test lets one fill land first: Low 2 started, a track by Codeine
// added. Radio caches the similar artists of an artist it asked and falls
// back on the artists of the tracks it played, so the seed of a fill is read
// from its first artist.getsimilar call, on an artist radio never asked.

// bedhead is a track by an artist radio never asked about.
var bedhead = Track{Path: "/elsewhere/Bedhead/1.mp3", Artist: "Bedhead", Title: "Bedhead 1"}

// fillSeed returns the artist the next fill asks Last.fm the similar artists of.
func fillSeed(t *testing.T, calls <-chan lastfmtest.Call) string {
	t.Helper()
	return waitCall(t, calls, similarMethod).Params.Get("artist")
}

// fallbackEnds waits for a fill seeded on Bedhead to end: Bedhead's similar
// artist, Codeine, is a recent seed, so the fill falls back on Codeine, the
// artist of the track radio played, which gives nothing either. A test that
// returned before would leave that call to reach the Last.fm of the next one.
func fallbackEnds(t *testing.T, c *Core, calls <-chan lastfmtest.Call) {
	t.Helper()
	if artist := waitCall(t, calls, similarMethod).Params.Get("artist"); artist != "Codeine" {
		t.Errorf("fallback artist = %q, want Codeine", artist)
	}
	settle(c) // the fill lands
}

func TestRadio_FillAfterTheQueueWasReplacedSeedsOnTheTrackPlaying(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, _ := newRadioCore(t)
	radioFilledOnce(t, c, calls)

	pb := c.Playback()
	pb.ReplaceTracks(bedhead)
	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}

	if seed := fillSeed(t, calls); seed != "Bedhead" {
		t.Errorf("seed artist = %q, want Bedhead", seed)
	}
	fallbackEnds(t, c, calls)
}

func TestRadio_FillOnAnAppendedTrackSeedsOnIt(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, _ := newRadioCore(t)
	radioFilledOnce(t, c, calls)

	c.Playback().AddTracks(bedhead)
	playAt(t, c, 3)

	if seed := fillSeed(t, calls); seed != "Bedhead" {
		t.Errorf("seed artist = %q, want Bedhead", seed)
	}
	fallbackEnds(t, c, calls)
}

func TestRadio_NewSessionSeedsOnTheTrackPlaying(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, _ := newRadioCore(t)
	radioFilledOnce(t, c, calls)

	pb := c.Playback()
	pb.CycleRepeatMode() // radio -> off
	if err := pb.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	pb.ClearQueue()
	pb.AddTracks(bedhead)
	radioOn(t, c) // with no current track
	settle(c)
	if err := pb.Play(); err != nil {
		t.Fatalf("play: %v", err)
	}

	if seed := fillSeed(t, calls); seed != "Bedhead" {
		t.Errorf("seed artist = %q, want Bedhead", seed)
	}
	// Radio forgot its recent seeds when it was left: Codeine is not one, the
	// fill adds its tracks.
	eventually(t, "radio adds tracks to the queue", func() bool { return pb.QueueLen() > 1 })
	settle(c)
}

func TestRadio_NearEndFillSeedsOnTheTrackPlaying(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, mock := newRadioCore(t)
	radioFilledOnce(t, c, calls)

	// Bedhead starts with a track after it, removed while it plays: it
	// becomes the last one without ever starting as the last one.
	pb := c.Playback()
	pb.AddTracks(bedhead, Track{Path: "/elsewhere/Other/1.mp3", Artist: "Other", Title: "Other 1"})
	playAt(t, c, 3)
	settle(c) // the core has seen the track start
	mock.SetDuration(3 * time.Minute)
	mock.SetPosition(time.Minute)
	pb.RemoveTracks([]int{4})
	expectNoCall(t, calls, similarMethod) // two minutes left

	mock.SetPosition(3*time.Minute - 10*time.Second)

	if seed := fillSeed(t, calls); seed != "Bedhead" {
		t.Errorf("seed artist = %q, want Bedhead", seed)
	}
	fallbackEnds(t, c, calls)
}

func TestRadio_TrackWithNoArtistStartsNoFill(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, _ := newRadioCore(t)
	radioFilledOnce(t, c, calls)

	c.Playback().AddTracks(Track{Path: "/elsewhere/untagged.mp3", Title: "Untagged"})
	playAt(t, c, 3)

	expectNoCall(t, calls, similarMethod)
}

func TestRadio_QueueLeftAloneSeedsOnTheTrackRadioAdded(t *testing.T) {
	calls := lastfmtest.Serve(t, codeineIsSimilar)
	c, _ := newRadioCore(t)
	radioFilledOnce(t, c, calls)

	playAt(t, c, 2)

	if seed := fillSeed(t, calls); seed != "Codeine" {
		t.Errorf("seed artist = %q, want Codeine", seed)
	}
	settle(c) // the fill lands: Codeine is a recent seed, it adds nothing
}
