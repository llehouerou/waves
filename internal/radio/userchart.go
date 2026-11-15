package radio

import (
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/library"
)

// userChartMaxPages is how many pages of the user's chart one refresh reads
// at most: past them, the least played tracks get no boost.
const userChartMaxPages = 30

// TrackKey is a track of the user's chart: its artist and its title, as
// library.NormalizeTitle leaves them.
type TrackKey struct {
	Artist string
	Title  string
}

// trackKey returns the key of the track by artist titled title.
func trackKey(artist, title string) TrackKey {
	return TrackKey{Artist: library.NormalizeTitle(artist), Title: library.NormalizeTitle(title)}
}

// userPlays returns the play counts of the tracks the user scrobbled, from
// the chart of the account linked: the one cached while it is fresh, or the
// one Last.fm gives now. None with no account linked, or when Last.fm could
// not give it: the plays only boost the score, and the next fill asks again.
//
// Last.fm refusing the chart is remembered as an empty chart: it refuses it
// however often it is asked.
func (r *Radio) userPlays() map[TrackKey]int {
	if r.client == nil {
		return nil
	}
	account := r.client.SessionID()
	if account == "" {
		return nil
	}
	if plays, ok, err := r.cache.GetUserChart(account); err == nil && ok {
		return plays
	}

	tracks, err := r.client.GetUserTopTracks(userChartMaxPages)
	if err != nil && !lastfm.IsRefusal(err) {
		return nil
	}
	if r.client.SessionID() != account {
		return nil // another account was linked meanwhile: not its chart
	}
	plays := make(map[TrackKey]int, len(tracks))
	for _, t := range tracks {
		// The same track under two spellings is one track, played as many
		// times as both.
		plays[trackKey(t.Artist, t.Name)] += t.Playcount
	}
	_ = r.cache.SetUserChart(account, plays) //nolint:errcheck // not stored: the next fill reads it again
	return plays
}
