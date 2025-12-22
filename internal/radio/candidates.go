package radio

import (
	"math/rand/v2"
	"sort"

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

// selectTracks selects tracks from candidates using weighted random selection.
// Returns up to count tracks, avoiding duplicates and enforcing artist variety.
// artistCounts tracks how many times each artist has appeared recently.
// maxArtistRepeat limits how many times the same artist can appear in the window.
func selectTracks(candidates []Candidate, count int, artistCounts map[string]int, maxArtistRepeat int) []Candidate {
	if len(candidates) == 0 {
		return nil
	}

	// Sort by score (descending) for better selection
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Calculate total score for probability distribution
	totalScore := 0.0
	for i := range candidates {
		totalScore += candidates[i].Score
	}

	if totalScore == 0 {
		// All scores are 0, use uniform distribution
		totalScore = float64(len(candidates))
		for i := range candidates {
			candidates[i].Score = 1.0
		}
	}

	selected := make([]Candidate, 0, count)
	used := make(map[string]bool)          // Track paths already selected
	sessionArtists := make(map[string]int) // Track artists selected in this batch

	// Maximum attempts to avoid infinite loops
	maxAttempts := count * 10

	for len(selected) < count && len(used) < len(candidates) && maxAttempts > 0 {
		maxAttempts--

		// Weighted random selection
		r := rand.Float64() * totalScore //nolint:gosec // crypto not needed for music selection
		cumulative := 0.0

		for i := range candidates {
			if used[candidates[i].LibraryTrack.Path] {
				continue
			}

			// Check artist variety limit (recent + this session)
			artist := candidates[i].LibraryTrack.Artist
			totalArtistCount := artistCounts[artist] + sessionArtists[artist]
			if totalArtistCount >= maxArtistRepeat {
				continue // Skip artists that have hit the limit
			}

			cumulative += candidates[i].Score
			if r <= cumulative {
				selected = append(selected, candidates[i])
				used[candidates[i].LibraryTrack.Path] = true
				sessionArtists[artist]++

				// Reduce total score for next iteration
				totalScore -= candidates[i].Score
				break
			}
		}
	}

	return selected
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
