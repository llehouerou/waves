package core

import "github.com/llehouerou/waves/internal/lyrics"

// The lyrics data a client handles, so that it imports the core only.
type (
	// TrackLyrics are the lyrics of a track, line by line. They are synced
	// when their lines carry the time they are sung at.
	TrackLyrics = lyrics.Lyrics
	LyricsLine  = lyrics.Line

	// LyricsTrack is the track lyrics are fetched for.
	LyricsTrack = lyrics.TrackInfo
)

// ErrNoLyrics is returned by Fetch for a track nobody has lyrics for.
var ErrNoLyrics = lyrics.ErrNotFound

// Lyrics is where the lyrics of a track come from: the .lrc file next to it,
// else those fetched earlier, else lrclib, whose answer is kept for the next
// time.
type Lyrics struct {
	source *lyrics.Source
	core   *Core
}

// Fetch returns the lyrics of a track, or ErrNoLyrics.
//
// It waits for the disk and for lrclib: a client calls it off its UI
// goroutine.
func (l *Lyrics) Fetch(track LyricsTrack) (*TrackLyrics, error) {
	ctx, cancel := l.core.untilStopped() // a quit does not wait for lrclib
	defer cancel()
	return l.source.Fetch(ctx, track)
}
