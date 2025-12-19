package radio

import (
	"strings"
	"unicode"

	"github.com/llehouerou/waves/internal/lastfm"
)

// matchArtists matches Last.fm similar artists to local library artists using fuzzy matching.
func matchArtists(similar []lastfm.SimilarArtist, localArtists []string, threshold float64) []MatchedArtist {
	var matched []MatchedArtist

	// Build normalized lookup map for local artists
	normalizedLocal := make(map[string]string) // normalized -> original
	for _, artist := range localArtists {
		norm := normalizeString(artist)
		normalizedLocal[norm] = artist
	}

	for _, sa := range similar {
		normSimilar := normalizeString(sa.Name)

		// Try exact match first
		if local, ok := normalizedLocal[normSimilar]; ok {
			matched = append(matched, MatchedArtist{
				LastfmArtist: sa,
				LocalArtist:  local,
			})
			continue
		}

		// Try fuzzy match
		bestMatch := ""
		bestScore := 0.0

		for norm, local := range normalizedLocal {
			score := similarity(normSimilar, norm)
			if score >= threshold && score > bestScore {
				bestScore = score
				bestMatch = local
			}
		}

		if bestMatch != "" {
			matched = append(matched, MatchedArtist{
				LastfmArtist: sa,
				LocalArtist:  bestMatch,
			})
		}
	}

	return matched
}

// normalizeString normalizes a string for comparison.
// Converts to lowercase, removes punctuation, and collapses whitespace.
func normalizeString(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Remove common suffixes/prefixes that cause mismatches
	s = strings.TrimSuffix(s, " (remastered)")
	s = strings.TrimSuffix(s, " (remaster)")
	s = strings.TrimSuffix(s, " - remastered")
	s = strings.TrimSuffix(s, " - remaster)")
	s = strings.TrimSuffix(s, " [remastered]")

	// Remove punctuation and normalize whitespace
	var result strings.Builder
	lastWasSpace := true // Start true to trim leading spaces

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
			lastWasSpace = false
		} else if unicode.IsSpace(r) || r == '-' || r == '_' {
			if !lastWasSpace {
				result.WriteRune(' ')
				lastWasSpace = true
			}
		}
		// Skip other punctuation
	}

	return strings.TrimSpace(result.String())
}

// similarity calculates the similarity between two strings using Levenshtein distance.
// Returns a value between 0 and 1, where 1 means identical.
func similarity(a, b string) float64 {
	if a == b {
		return 1.0
	}

	lenA := len(a)
	lenB := len(b)

	if lenA == 0 || lenB == 0 {
		return 0.0
	}

	// Calculate Levenshtein distance
	dist := levenshteinDistance(a, b)

	// Convert to similarity (0-1 range)
	maxLen := max(lenA, lenB)

	return 1.0 - float64(dist)/float64(maxLen)
}

// levenshteinDistance calculates the edit distance between two strings.
func levenshteinDistance(a, b string) int {
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}

	// Convert to rune slices for proper unicode handling
	runesA := []rune(a)
	runesB := []rune(b)

	lenA := len(runesA)
	lenB := len(runesB)

	// Create distance matrix (only need two rows)
	prev := make([]int, lenB+1)
	curr := make([]int, lenB+1)

	// Initialize first row
	for j := 0; j <= lenB; j++ {
		prev[j] = j
	}

	// Fill in the rest
	for i := 1; i <= lenA; i++ {
		curr[0] = i

		for j := 1; j <= lenB; j++ {
			cost := 1
			if runesA[i-1] == runesB[j-1] {
				cost = 0
			}

			curr[j] = min(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}

		// Swap rows
		prev, curr = curr, prev
	}

	return prev[lenB]
}
