package core

import (
	"time"

	"github.com/llehouerou/waves/internal/errmsg"
	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/radio"
)

// radioNearEnd is how close to the end of the last track the queue is filled
// when nothing filled it earlier.
const radioNearEnd = 15 * time.Second

// CycleRepeatMode moves to the next repeat mode and returns it:
// Off -> All -> One -> Radio -> Off. Radio is skipped unless Last.fm is linked.
func (p *Playback) CycleRepeatMode() RepeatMode {
	current := p.RepeatMode()
	next := RepeatOff
	switch current {
	case RepeatOff:
		next = RepeatAll
	case RepeatAll:
		next = RepeatOne
	case RepeatOne:
		if p.core.radio != nil && p.core.lastfmLinked() {
			next = RepeatRadio
		}
	case RepeatRadio:
	}

	if r := p.core.radio; r != nil {
		switch {
		case current == RepeatRadio:
			r.Disable()
		case next == RepeatRadio:
			r.Enable()
			if track := p.CurrentTrack(); track != nil {
				r.SetSeed(track.Artist)
			}
		}
	}

	p.SetRepeatMode(next)
	return next
}

// radioActive reports whether the queue is to be kept filled by radio.
func (c *Core) radioActive() bool {
	return c.radio != nil && c.playback.RepeatMode() == RepeatRadio
}

// fillRadioOnLastTrack fetches more tracks when the last track of the queue
// starts, so that they are there before it ends.
func (c *Core) fillRadioOnLastTrack() {
	pb := c.playback
	if !c.radioActive() || pb.QueueIsEmpty() || pb.QueueCurrentIndex() < pb.QueueLen()-1 {
		return
	}
	c.startRadioFill()
}

// fillRadioNearEnd fetches more tracks when the track playing is about to end
// with nothing after it: it became the last one after it started, because the
// queue was edited. Called by the loop on every clock tick, it fills once per
// play.
func (c *Core) fillRadioNearEnd() {
	pb := c.playback
	if !c.radioActive() || c.play == nil || c.play.radioFilled || pb.QueueHasNext() {
		return
	}
	duration, position := pb.Duration(), pb.Position()
	if duration <= 0 || position <= 0 || duration-position > radioNearEnd {
		return
	}
	c.play.radioFilled = c.startRadioFill()
}

// startRadioFill asks radio for tracks similar to the seed artist, off the
// loop: the answer comes back on c.fills. It reports whether a fill started.
func (c *Core) startRadioFill() bool {
	if c.filling {
		return false
	}

	seed := c.radio.CurrentSeed()
	if seed == "" {
		if track := c.playback.CurrentTrack(); track != nil {
			seed = track.Artist
			c.radio.SetSeed(seed)
		}
	}
	if seed == "" {
		return false
	}

	// Favorites only boost the score: without them the fill is still good.
	var favorites map[int64]bool
	if c.playlists != nil {
		favorites, _ = c.playlists.FavoriteTrackIDs() //nolint:errcheck // see above
	}

	c.filling = true
	r := c.radio
	go func() { c.fills <- r.Fill(seed, favorites) }()
	return true
}

// radioFilled puts the tracks radio found at the end of the queue, and starts
// playback when the queue ran out while they were being fetched.
func (c *Core) radioFilled(res radio.FillResult) {
	c.filling = false
	if res.Err != nil {
		c.emit(Failed{Op: errmsg.OpRadioFill, Err: res.Err})
		return
	}
	if len(res.Tracks) == 0 {
		return
	}

	tracks := playback.TracksFromPlaylist(res.Tracks)
	pb := c.playback
	pb.AddTracks(tracks...)

	// The seed moves to the last track added, and radio remembers what it
	// gave to avoid repeating itself.
	c.radio.SetSeed(tracks[len(tracks)-1].Artist)
	for _, track := range tracks {
		c.radio.AddToRecentlyPlayed(track.Path, track.Artist)
	}

	if pb.IsStopped() {
		if err := pb.Play(); err != nil {
			c.emit(Failed{Op: errmsg.OpPlaybackStart, Err: err})
		}
	}
}
