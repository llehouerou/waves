package search

import (
	"sort"
	"strings"
	"unicode"
)

// Match represents a search match with its index and score.
type Match struct {
	Index int
	Score float64
}

// TrigramMatcher performs trigram-based search with multi-word support.
type TrigramMatcher struct {
	items        []Item
	itemTrigrams []map[string]struct{}
	normalized   []string
}

// NewTrigramMatcher creates a matcher for the given items.
func NewTrigramMatcher(items []Item) *TrigramMatcher {
	m := &TrigramMatcher{
		items:        items,
		itemTrigrams: make([]map[string]struct{}, len(items)),
		normalized:   make([]string, len(items)),
	}

	for i, item := range items {
		text := normalize(item.FilterValue())
		m.normalized[i] = text
		m.itemTrigrams[i] = generateTrigrams(text)
	}

	return m
}

// Search finds items matching the query.
// Query is split into words, each word must match (AND logic).
// Returns matches sorted by score (best first).
func (m *TrigramMatcher) Search(query string) []Match {
	query = normalize(query)
	if query == "" {
		// Return all items with zero score
		matches := make([]Match, len(m.items))
		for i := range m.items {
			matches[i] = Match{Index: i}
		}
		return matches
	}

	words := strings.Fields(query)
	if len(words) == 0 {
		matches := make([]Match, len(m.items))
		for i := range m.items {
			matches[i] = Match{Index: i}
		}
		return matches
	}

	// Generate trigrams for each query word
	wordTrigrams := make([]map[string]struct{}, len(words))
	for i, word := range words {
		wordTrigrams[i] = generateTrigrams(word)
	}

	var matches []Match

	for i, itemTris := range m.itemTrigrams {
		score := m.scoreItem(i, words, wordTrigrams, itemTris)
		if score > 0 {
			matches = append(matches, Match{Index: i, Score: score})
		}
	}

	// Sort by score descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	return matches
}

// scoreItem calculates how well an item matches the query words.
// All words must match for a non-zero score.
func (m *TrigramMatcher) scoreItem(idx int, words []string, wordTrigrams []map[string]struct{}, itemTris map[string]struct{}) float64 {
	text := m.normalized[idx]
	totalScore := 0.0

	for i, word := range words {
		wordTris := wordTrigrams[i]

		// For short words (1-2 chars), use substring match
		if len(word) <= 2 {
			if !strings.Contains(text, word) {
				return 0 // Word not found, no match
			}
			totalScore += 1.0
			continue
		}

		// Calculate how many query trigrams are found in the item
		// Using coverage (intersection/query size) instead of Jaccard
		// because Jaccard penalizes short queries against long texts
		coverage := trigramCoverage(wordTris, itemTris)
		if coverage < 0.4 {
			return 0 // Word doesn't match well enough
		}
		similarity := coverage

		// Bonus for exact substring match
		if strings.Contains(text, word) {
			similarity += 0.5
		}

		totalScore += similarity
	}

	// Normalize by number of words
	return totalScore / float64(len(words))
}

// normalize lowercases and removes diacritics for matching.
func normalize(s string) string {
	s = strings.ToLower(s)
	// Simple normalization - just lowercase for now
	// Could add unicode normalization if needed
	return s
}

// generateTrigrams creates the set of trigrams for a string.
// Pads with spaces at start/end for better prefix/suffix matching.
func generateTrigrams(s string) map[string]struct{} {
	if s == "" {
		return nil
	}

	tris := make(map[string]struct{})

	// Pad string for prefix/suffix trigrams
	padded := "  " + s + "  "
	runes := []rune(padded)

	for i := 0; i <= len(runes)-3; i++ {
		tri := string(runes[i : i+3])
		// Skip trigrams that are all whitespace
		if strings.TrimSpace(tri) != "" {
			tris[tri] = struct{}{}
		}
	}

	return tris
}

// trigramCoverage calculates what fraction of query trigrams are found in the item.
// Returns |A ∩ B| / |A| - better for partial word matching than Jaccard.
func trigramCoverage(query, item map[string]struct{}) float64 {
	if len(query) == 0 {
		return 0
	}

	intersection := 0
	for tri := range query {
		if _, ok := item[tri]; ok {
			intersection++
		}
	}

	return float64(intersection) / float64(len(query))
}

// RemoveDiacritics removes accents from characters.
// Useful for searching "cafe" to match "café".
func RemoveDiacritics(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			// Skip combining marks (diacritics)
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
