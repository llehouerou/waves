package radio

import (
	"math/rand/v2"
	"sort"

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
	RecentlyPlayed  bool    // Whether track was recently played in session
	Score           float64 // Final calculated score
}

// selectTracks selects tracks from candidates using weighted random selection.
// Returns up to count tracks, avoiding duplicates.
func selectTracks(candidates []Candidate, count int) []Candidate {
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
	used := make(map[string]bool) // Track paths already selected

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

			cumulative += candidates[i].Score
			if r <= cumulative {
				selected = append(selected, candidates[i])
				used[candidates[i].LibraryTrack.Path] = true

				// Reduce total score for next iteration
				totalScore -= candidates[i].Score
				break
			}
		}
	}

	return selected
}
