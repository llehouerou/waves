package core

import (
	"time"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/state"
	"github.com/llehouerou/waves/internal/tags"
)

// Last.fm's rules: a track counts once half of it or four minutes have been
// played, whichever comes first, and only if it is at least 30 seconds long.
const (
	scrobbleMinDuration  = 30 * time.Second
	scrobbleMaxThreshold = 4 * time.Minute
)

// lastfmLinked reports whether a Last.fm account is linked.
func (c *Core) lastfmLinked() bool {
	return c.lastfm.client != nil && c.lastfm.client.IsAuthenticated()
}

// checkScrobble scrobbles the current play once it has passed the threshold.
// Called by the loop on every clock tick while a track plays.
func (c *Core) checkScrobble() {
	if c.play == nil || c.play.scrobbled || !c.lastfmLinked() {
		return
	}

	duration := c.playback.Duration()
	if duration < scrobbleMinDuration {
		return
	}
	if c.playback.Position() < min(duration/2, scrobbleMaxThreshold) {
		return
	}
	c.play.scrobbled = true

	info := c.playback.TrackInfo()
	if info == nil {
		return
	}
	track := playedTrack(info, duration)
	track.Timestamp = c.play.startedAt
	// The loop has ended before Close waits for the background work: a
	// scrobble it starts is always waited for.
	c.work.Go(func() { c.submitScrobble(track) })
}

// playedTrack is a track as Last.fm is told of it, by the scrobble of its
// play and by the "now playing" update of its start: what its file says
// (info), not what the queue shows.
func playedTrack(info *tags.FileInfo, duration time.Duration) lastfm.PlayedTrack {
	track := lastfm.PlayedTrack{
		Artist:        info.Artist,
		Track:         info.Title,
		Album:         info.Album,
		Duration:      duration,
		MBRecordingID: info.MBRecordingID,
	}
	if info.AlbumArtist != info.Artist {
		track.AlbumArtist = info.AlbumArtist
	}
	return track
}

// submitScrobble sends a scrobble to Last.fm, and keeps it as pending when
// Last.fm does not take it.
//
// Close waits for Last.fm to answer, scrobbleCloseTicks at most, the same
// ticks as for a pass over the pending scrobbles: past them the scrobble is
// kept as pending, as if Last.fm had refused it, and the next run sends it.
// Last.fm may have taken it all the same, and gets it twice.
func (c *Core) submitScrobble(track lastfm.PlayedTrack) {
	ctx, cancel := c.untilStoppedFor(scrobbleCloseTicks * c.tick)
	defer cancel()
	if err := c.lastfm.client.Scrobble(ctx, track); err == nil {
		return
	}
	err := c.state.AddPendingScrobble(state.PendingScrobble{
		Artist:        track.Artist,
		Track:         track.Track,
		Album:         track.Album,
		AlbumArtist:   track.AlbumArtist,
		DurationSecs:  int(track.Duration.Seconds()),
		Timestamp:     track.Timestamp,
		MBRecordingID: track.MBRecordingID,
	})
	if err != nil {
		c.emit(Failed{Op: errmsg.OpLastfmScrobble, Err: err})
	}
}
