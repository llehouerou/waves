package radio

import (
	"math/rand/v2"

	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/library"
)

// Candidate represents a potential track to add to the queue.
type Candidate struct {
	LibraryTrack    library.Track
	SimilarityScore float64 // 0-1 from Last.fm
	GlobalPlaycount int     // From Last.fm artist.getTopTracks
	Rank            int     // Rank in top tracks
	UserScrobbled   bool    // Whether user has scrobbled this track
	UserPlaycount   int     // User's scrobble count for this track
	IsFavorite      bool    // Whether track is in user's Favorites playlist
	RecentlyPlayed  bool    // Whether track was recently played in session
	Score           float64 // Final calculated score
}

// selectTracks selects up to count tracks from candidates, one draw at a time.
// A candidate is selectable while its track is not selected yet (the same
// track may be among the candidates twice) and its artist is
// under the limit: its count in artistCounts, the artists played recently,
// plus its tracks already selected here is below maxArtistRepeat. Each draw
// is made among the selectable candidates only (drawSelectable), so
// selectTracks returns count tracks whenever that many can be selected.
func selectTracks(candidates []Candidate, count int, artistCounts map[string]int, maxArtistRepeat int) []Candidate {
	if len(candidates) == 0 {
		return nil
	}

	selected := make([]Candidate, 0, count)
	used := make(map[string]bool)          // paths of the tracks selected here
	sessionArtists := make(map[string]int) // tracks selected here, by artist
	selectable := func(i int) bool {
		artist := candidates[i].LibraryTrack.Artist
		return !used[candidates[i].LibraryTrack.Path] && artistCounts[artist]+sessionArtists[artist] < maxArtistRepeat
	}

	for len(selected) < count {
		i := drawSelectable(candidates, selectable)
		if i < 0 {
			break
		}
		selected = append(selected, candidates[i])
		used[candidates[i].LibraryTrack.Path] = true
		sessionArtists[candidates[i].LibraryTrack.Artist]++
	}

	return selected
}

// drawSelectable draws the index of a candidate among those selectable says
// it may select, or returns -1 when there is none. While one of them has a
// score above 0, each with a score is drawn with a probability of its score
// over their sum, and none scored 0 is; otherwise one is drawn uniformly.
func drawSelectable(candidates []Candidate, selectable func(int) bool) int {
	total := 0.0
	var zero []int
	last := -1 // the last selectable candidate with a score
	for i := range candidates {
		if !selectable(i) {
			continue
		}
		if candidates[i].Score > 0 {
			total += candidates[i].Score
			last = i
		} else {
			zero = append(zero, i)
		}
	}

	if last < 0 {
		if len(zero) == 0 {
			return -1
		}
		return zero[rand.IntN(len(zero))] //nolint:gosec // crypto not needed for music selection
	}

	r := rand.Float64() * total //nolint:gosec // crypto not needed for music selection
	cumulative := 0.0
	for i := range candidates {
		if !selectable(i) || candidates[i].Score <= 0 {
			continue
		}
		cumulative += candidates[i].Score
		if r < cumulative {
			return i
		}
	}
	// r rounded up to total: the draw lands on the last one.
	return last
}

// selectArtistsWeighted selects artists using weighted random selection based on similarity score.
// Higher similarity = higher chance, but lower-similarity artists still have a shot.
// This breaks cross-session determinism where the same corridor of artists would always be selected.
func selectArtistsWeighted(artists []MatchedArtist, count int) []MatchedArtist {
	if len(artists) == 0 {
		return nil
	}

	if count >= len(artists) {
		// Need all of them, just return as-is
		result := make([]MatchedArtist, len(artists))
		copy(result, artists)
		return result
	}

	// Calculate total weight from similarity scores
	totalWeight := 0.0
	weights := make([]float64, len(artists))
	for i, a := range artists {
		// Use similarity score as weight, with a minimum floor
		weight := a.LastfmArtist.MatchScore
		if weight < 0.1 {
			weight = 0.1
		}
		weights[i] = weight
		totalWeight += weight
	}

	selected := make([]MatchedArtist, 0, count)
	used := make(map[string]bool)

	// Maximum attempts to avoid infinite loops
	maxAttempts := count * 10

	for len(selected) < count && len(used) < len(artists) && maxAttempts > 0 {
		maxAttempts--

		// Weighted random selection
		r := rand.Float64() * totalWeight //nolint:gosec // crypto not needed for music selection
		cumulative := 0.0

		for i := range artists {
			if used[artists[i].LocalArtist] {
				continue
			}

			cumulative += weights[i]
			if r <= cumulative {
				selected = append(selected, artists[i])
				used[artists[i].LocalArtist] = true
				totalWeight -= weights[i]
				break
			}
		}
	}

	return selected
}

// SimilarArtist is an alias to avoid import in callers.
type SimilarArtist = lastfm.SimilarArtist
