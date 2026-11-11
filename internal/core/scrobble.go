package core

import (
	"time"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/state"
)

// Last.fm's rules: a track counts once half of it or four minutes have been
// played, whichever comes first, and only if it is at least 30 seconds long.
const (
	scrobbleMinDuration  = 30 * time.Second
	scrobbleMaxThreshold = 4 * time.Minute
)

// lastfmLinked reports whether Last.fm is configured and authenticated.
func (c *Core) lastfmLinked() bool {
	return c.lastfm != nil && c.lastfm.IsAuthenticated()
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

	// What is scrobbled is what the file says, not what the queue shows.
	info := c.playback.TrackInfo()
	if info == nil {
		return
	}
	track := lastfm.ScrobbleTrack{
		Artist:        info.Artist,
		Track:         info.Title,
		Album:         info.Album,
		Duration:      duration,
		Timestamp:     c.play.startedAt,
		MBRecordingID: info.MBRecordingID,
	}
	if info.AlbumArtist != info.Artist {
		track.AlbumArtist = info.AlbumArtist
	}
	go c.submitScrobble(track)
}

// submitScrobble sends a scrobble to Last.fm, and keeps it as pending when
// Last.fm does not take it.
func (c *Core) submitScrobble(track lastfm.ScrobbleTrack) {
	if err := c.lastfm.Scrobble(track); err == nil {
		return
	}
	err := c.state.AddPendingScrobble(state.PendingScrobble{
		Artist:        track.Artist,
		Track:         track.Track,
		Album:         track.Album,
		DurationSecs:  int(track.Duration.Seconds()),
		Timestamp:     track.Timestamp,
		MBRecordingID: track.MBRecordingID,
	})
	if err != nil {
		c.emit(Failed{Op: errmsg.OpLastfmScrobble, Err: err})
	}
}
