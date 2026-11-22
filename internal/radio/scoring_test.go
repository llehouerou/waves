//nolint:goconst // test files commonly repeat strings for test data
package radio

import (
	"fmt"
	"slices"
	"testing"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/library"
)

// Test levenshteinDistance

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "adc", 1},
		{"abc", "dbc", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"abc", "xyz", 3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := levenshteinDistance(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// Test similarity

func TestSimilarity(t *testing.T) {
	tests := []struct {
		a, b    string
		wantMin float64
		wantMax float64
	}{
		{"abc", "abc", 1.0, 1.0},
		{"abc", "", 0.0, 0.0},
		{"", "abc", 0.0, 0.0},
		{"abc", "abd", 0.6, 0.7}, // 1 edit in 3 chars
		{"radiohead", "radiohedd", 0.8, 0.9},
		{"the beatles", "beatles", 0.6, 0.7},
		{"completely", "different", 0.0, 0.3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := similarity(tt.a, tt.b)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("similarity(%q, %q) = %f, want between %f and %f", tt.a, tt.b, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestSimilarity_Symmetric(t *testing.T) {
	pairs := [][2]string{
		{"abc", "abd"},
		{"radiohead", "radiohedd"},
		{"the beatles", "beatles"},
	}

	for _, pair := range pairs {
		a, b := pair[0], pair[1]
		t.Run(a+"_"+b, func(t *testing.T) {
			s1 := similarity(a, b)
			s2 := similarity(b, a)
			if s1 != s2 {
				t.Errorf("similarity not symmetric: (%q, %q) = %f, (%q, %q) = %f", a, b, s1, b, a, s2)
			}
		})
	}
}

// Test matchArtists

func TestMatchArtists_ExactMatch(t *testing.T) {
	similar := []lastfm.SimilarArtist{
		{Name: "Radiohead", MatchScore: 0.9},
		{Name: "Muse", MatchScore: 0.8},
	}
	localArtists := []string{"Radiohead", "Muse", "Coldplay"}

	matched := matchArtists(similar, localArtists, 0.8)

	if len(matched) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matched))
	}
	if matched[0].LocalArtist != "Radiohead" {
		t.Errorf("first match = %q, want Radiohead", matched[0].LocalArtist)
	}
	if matched[1].LocalArtist != "Muse" {
		t.Errorf("second match = %q, want Muse", matched[1].LocalArtist)
	}
}

func TestMatchArtists_CaseInsensitive(t *testing.T) {
	similar := []lastfm.SimilarArtist{
		{Name: "RADIOHEAD", MatchScore: 0.9},
	}
	localArtists := []string{"radiohead", "Muse"}

	matched := matchArtists(similar, localArtists, 0.8)

	if len(matched) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matched))
	}
	if matched[0].LocalArtist != "radiohead" {
		t.Errorf("match = %q, want radiohead", matched[0].LocalArtist)
	}
}

func TestMatchArtists_FuzzyMatch(t *testing.T) {
	similar := []lastfm.SimilarArtist{
		{Name: "Guns N' Roses", MatchScore: 0.9},
	}
	// Local library has slightly different name
	localArtists := []string{"Guns N Roses", "Metallica"}

	matched := matchArtists(similar, localArtists, 0.8)

	if len(matched) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matched))
	}
	if matched[0].LocalArtist != "Guns N Roses" {
		t.Errorf("match = %q, want Guns N Roses", matched[0].LocalArtist)
	}
}

func TestMatchArtists_NoMatch(t *testing.T) {
	similar := []lastfm.SimilarArtist{
		{Name: "Unknown Artist", MatchScore: 0.9},
	}
	localArtists := []string{"Radiohead", "Muse"}

	matched := matchArtists(similar, localArtists, 0.8)

	if len(matched) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matched))
	}
}

func TestMatchArtists_ThresholdFiltering(t *testing.T) {
	similar := []lastfm.SimilarArtist{
		{Name: "The Beatles", MatchScore: 0.9},
	}
	// "Beatles" is close but not exact - depends on threshold
	localArtists := []string{"Beatles"}

	// High threshold should reject
	matchedHigh := matchArtists(similar, localArtists, 0.95)
	if len(matchedHigh) != 0 {
		t.Errorf("with threshold 0.95, expected 0 matches, got %d", len(matchedHigh))
	}

	// Lower threshold should accept
	matchedLow := matchArtists(similar, localArtists, 0.6)
	if len(matchedLow) != 1 {
		t.Errorf("with threshold 0.6, expected 1 match, got %d", len(matchedLow))
	}
}

// Test countArtists

func TestCountArtists(t *testing.T) {
	artists := []string{"Radiohead", "Muse", "Radiohead", "Coldplay", "Radiohead", "Muse"}
	counts := countArtists(artists)

	if counts["Radiohead"] != 3 {
		t.Errorf("Radiohead count = %d, want 3", counts["Radiohead"])
	}
	if counts["Muse"] != 2 {
		t.Errorf("Muse count = %d, want 2", counts["Muse"])
	}
	if counts["Coldplay"] != 1 {
		t.Errorf("Coldplay count = %d, want 1", counts["Coldplay"])
	}
	if counts["Unknown"] != 0 {
		t.Errorf("Unknown count = %d, want 0", counts["Unknown"])
	}
}

func TestCountArtists_Empty(t *testing.T) {
	counts := countArtists(nil)
	if len(counts) != 0 {
		t.Errorf("expected empty map, got %d entries", len(counts))
	}
}

// Test buildTopTrackMap

func TestBuildTopTrackMap(t *testing.T) {
	tracks := []lastfm.TopTrack{
		{Name: "Creep", Playcount: 1000000, Rank: 1},
		{Name: "Karma Police", Playcount: 800000, Rank: 2},
		{Name: "Paranoid Android", Playcount: 600000, Rank: 3},
	}

	m := buildTopTrackMap(tracks)

	// Check normalized lookup
	if tt, ok := m["creep"]; !ok || tt.Rank != 1 {
		t.Error("expected to find 'creep' with rank 1")
	}
	if tt, ok := m["karma police"]; !ok || tt.Rank != 2 {
		t.Error("expected to find 'karma police' with rank 2")
	}
	if _, ok := m["unknown"]; ok {
		t.Error("did not expect to find 'unknown'")
	}
}

// Test calculateScore

func TestCalculateScore_BaseScore(t *testing.T) {
	r := &Radio{
		config: config.RadioConfig{
			TopTrackBoost:       3.0,
			UserBoost:           1.3,
			FavoriteBoost:       2.0,
			DecayFactor:         0.1,
			MinSimilarityWeight: 0.1,
		},
	}

	// Track with no special attributes - just base score from playcount
	c := Candidate{
		GlobalPlaycount: 5000000, // 5M plays -> 0.5 base score
		SimilarityScore: 1.0,
	}

	score := r.calculateScore(c)

	// Expected: 0.5 * 1.0 (no top track boost) * 1.0 (no preference) * 1.0 (no decay) * 1.0 (similarity)
	if score < 0.45 || score > 0.55 {
		t.Errorf("base score = %f, expected around 0.5", score)
	}
}

func TestCalculateScore_MaxBaseScore(t *testing.T) {
	r := &Radio{
		config: config.RadioConfig{
			TopTrackBoost:       3.0,
			UserBoost:           1.3,
			FavoriteBoost:       2.0,
			DecayFactor:         0.1,
			MinSimilarityWeight: 0.1,
		},
	}

	// Very high playcount should cap at 1.0
	c := Candidate{
		GlobalPlaycount: 50000000, // 50M plays -> should cap at 1.0
		SimilarityScore: 1.0,
	}

	score := r.calculateScore(c)

	// Base score capped at 1.0
	if score < 0.95 || score > 1.05 {
		t.Errorf("capped score = %f, expected around 1.0", score)
	}
}

func TestCalculateScore_MinBaseScore(t *testing.T) {
	r := &Radio{
		config: config.RadioConfig{
			TopTrackBoost:       3.0,
			UserBoost:           1.3,
			FavoriteBoost:       2.0,
			DecayFactor:         0.1,
			MinSimilarityWeight: 0.1,
		},
	}

	// Zero playcount should get minimum base score
	c := Candidate{
		GlobalPlaycount: 0,
		SimilarityScore: 1.0,
	}

	score := r.calculateScore(c)

	// Minimum base score is 0.01
	if score < 0.009 || score > 0.011 {
		t.Errorf("min score = %f, expected around 0.01", score)
	}
}

func TestCalculateScore_TopTrackBoost(t *testing.T) {
	r := &Radio{
		config: config.RadioConfig{
			TopTrackBoost:       3.0,
			UserBoost:           1.3,
			FavoriteBoost:       2.0,
			DecayFactor:         0.1,
			MinSimilarityWeight: 0.1,
		},
	}

	// Track with rank 1 should get highest boost
	c1 := Candidate{
		GlobalPlaycount: 5000000,
		Rank:            1,
		SimilarityScore: 1.0,
	}

	// Track with rank 10 should get smaller boost
	c10 := Candidate{
		GlobalPlaycount: 5000000,
		Rank:            10,
		SimilarityScore: 1.0,
	}

	// Track with no rank
	c0 := Candidate{
		GlobalPlaycount: 5000000,
		Rank:            0,
		SimilarityScore: 1.0,
	}

	score1 := r.calculateScore(c1)
	score10 := r.calculateScore(c10)
	score0 := r.calculateScore(c0)

	// Rank 1 should have highest score
	if score1 <= score10 {
		t.Errorf("rank 1 score (%f) should be > rank 10 score (%f)", score1, score10)
	}
	if score10 <= score0 {
		t.Errorf("rank 10 score (%f) should be > no rank score (%f)", score10, score0)
	}
}

func TestCalculateScore_FavoriteBoost(t *testing.T) {
	r := &Radio{
		config: config.RadioConfig{
			TopTrackBoost:       3.0,
			UserBoost:           1.3,
			FavoriteBoost:       2.0,
			DecayFactor:         0.1,
			MinSimilarityWeight: 0.1,
		},
	}

	base := Candidate{
		GlobalPlaycount: 5000000,
		SimilarityScore: 1.0,
	}

	scrobbled := Candidate{
		GlobalPlaycount: 5000000,
		UserScrobbled:   true,
		SimilarityScore: 1.0,
	}

	favorite := Candidate{
		GlobalPlaycount: 5000000,
		IsFavorite:      true,
		SimilarityScore: 1.0,
	}

	// Favorite and scrobbled - favorite should take priority
	both := Candidate{
		GlobalPlaycount: 5000000,
		UserScrobbled:   true,
		IsFavorite:      true,
		SimilarityScore: 1.0,
	}

	scoreBase := r.calculateScore(base)
	scoreScrobbled := r.calculateScore(scrobbled)
	scoreFavorite := r.calculateScore(favorite)
	scoreBoth := r.calculateScore(both)

	// Favorite > Scrobbled > Base
	if scoreFavorite <= scoreScrobbled {
		t.Errorf("favorite score (%f) should be > scrobbled score (%f)", scoreFavorite, scoreScrobbled)
	}
	if scoreScrobbled <= scoreBase {
		t.Errorf("scrobbled score (%f) should be > base score (%f)", scoreScrobbled, scoreBase)
	}

	// Both should equal favorite (favorite takes priority)
	if scoreBoth != scoreFavorite {
		t.Errorf("both score (%f) should equal favorite score (%f)", scoreBoth, scoreFavorite)
	}
}

func TestCalculateScore_DecayPenalty(t *testing.T) {
	r := &Radio{
		config: config.RadioConfig{
			TopTrackBoost:       3.0,
			UserBoost:           1.3,
			FavoriteBoost:       2.0,
			DecayFactor:         0.1,
			MinSimilarityWeight: 0.1,
		},
	}

	fresh := Candidate{
		GlobalPlaycount: 5000000,
		SimilarityScore: 1.0,
	}

	recent := Candidate{
		GlobalPlaycount: 5000000,
		RecentlyPlayed:  true,
		SimilarityScore: 1.0,
	}

	scoreFresh := r.calculateScore(fresh)
	scoreRecent := r.calculateScore(recent)

	// Recently played should be penalized (10% of fresh)
	expectedRatio := r.config.DecayFactor
	actualRatio := scoreRecent / scoreFresh

	if actualRatio < expectedRatio-0.01 || actualRatio > expectedRatio+0.01 {
		t.Errorf("decay ratio = %f, expected %f", actualRatio, expectedRatio)
	}
}

func TestCalculateScore_SimilarityWeight(t *testing.T) {
	r := &Radio{
		config: config.RadioConfig{
			TopTrackBoost:       3.0,
			UserBoost:           1.3,
			FavoriteBoost:       2.0,
			DecayFactor:         0.1,
			MinSimilarityWeight: 0.1,
		},
	}

	high := Candidate{
		GlobalPlaycount: 5000000,
		SimilarityScore: 1.0,
	}

	low := Candidate{
		GlobalPlaycount: 5000000,
		SimilarityScore: 0.5,
	}

	veryLow := Candidate{
		GlobalPlaycount: 5000000,
		SimilarityScore: 0.05, // Below minimum
	}

	scoreHigh := r.calculateScore(high)
	scoreLow := r.calculateScore(low)
	scoreVeryLow := r.calculateScore(veryLow)

	// Higher similarity should mean higher score
	if scoreHigh <= scoreLow {
		t.Errorf("high similarity score (%f) should be > low similarity score (%f)", scoreHigh, scoreLow)
	}

	// Very low should be clamped to minimum
	// veryLow uses 0.1 (min) instead of 0.05, so it should be same as if similarity was 0.1
	minSim := Candidate{
		GlobalPlaycount: 5000000,
		SimilarityScore: 0.1,
	}
	scoreMinSim := r.calculateScore(minSim)

	if scoreVeryLow != scoreMinSim {
		t.Errorf("very low similarity score (%f) should equal min similarity score (%f)", scoreVeryLow, scoreMinSim)
	}
}

// Test selectTracks

func TestSelectTracks_Basic(t *testing.T) {
	candidates := []Candidate{
		{LibraryTrack: library.Track{Path: "/a.mp3", Artist: "Artist A"}, Score: 1.0},
		{LibraryTrack: library.Track{Path: "/b.mp3", Artist: "Artist B"}, Score: 0.5},
		{LibraryTrack: library.Track{Path: "/c.mp3", Artist: "Artist C"}, Score: 0.25},
	}

	selected := selectTracks(candidates, 2, nil, 10)

	if len(selected) != 2 {
		t.Errorf("expected 2 selected, got %d", len(selected))
	}
}

func TestSelectTracks_NoDuplicates(t *testing.T) {
	candidates := []Candidate{
		{LibraryTrack: library.Track{Path: "/a.mp3", Artist: "Artist A"}, Score: 1.0},
		{LibraryTrack: library.Track{Path: "/b.mp3", Artist: "Artist B"}, Score: 0.9},
	}

	// Request more than available
	selected := selectTracks(candidates, 5, nil, 10)

	if len(selected) != 2 {
		t.Errorf("expected 2 selected (no duplicates), got %d", len(selected))
	}

	// Check no duplicates
	paths := make(map[string]bool)
	for _, s := range selected {
		if paths[s.LibraryTrack.Path] {
			t.Errorf("duplicate track selected: %s", s.LibraryTrack.Path)
		}
		paths[s.LibraryTrack.Path] = true
	}
}

func TestSelectTracks_ArtistVariety(t *testing.T) {
	// All tracks from same artist
	candidates := []Candidate{
		{LibraryTrack: library.Track{Path: "/a.mp3", Artist: "Same Artist"}, Score: 1.0},
		{LibraryTrack: library.Track{Path: "/b.mp3", Artist: "Same Artist"}, Score: 0.9},
		{LibraryTrack: library.Track{Path: "/c.mp3", Artist: "Same Artist"}, Score: 0.8},
		{LibraryTrack: library.Track{Path: "/d.mp3", Artist: "Same Artist"}, Score: 0.7},
		{LibraryTrack: library.Track{Path: "/e.mp3", Artist: "Different Artist"}, Score: 0.1},
	}

	// Max 2 per artist
	selected := selectTracks(candidates, 4, nil, 2)

	// Should get at most 2 from "Same Artist" and 1 from "Different Artist"
	artistCount := make(map[string]int)
	for _, s := range selected {
		artistCount[s.LibraryTrack.Artist]++
	}

	if artistCount["Same Artist"] > 2 {
		t.Errorf("Same Artist count = %d, expected <= 2", artistCount["Same Artist"])
	}
}

func TestSelectTracks_RespectsRecentArtists(t *testing.T) {
	candidates := []Candidate{
		{LibraryTrack: library.Track{Path: "/a.mp3", Artist: "Recent Artist"}, Score: 1.0},
		{LibraryTrack: library.Track{Path: "/b.mp3", Artist: "Recent Artist"}, Score: 0.9},
		{LibraryTrack: library.Track{Path: "/c.mp3", Artist: "Fresh Artist"}, Score: 0.5},
	}

	// "Recent Artist" already played once
	artistCounts := map[string]int{"Recent Artist": 1}

	// Max 2 per artist - so "Recent Artist" can only have 1 more
	selected := selectTracks(candidates, 3, artistCounts, 2)

	artistCount := make(map[string]int)
	for _, s := range selected {
		artistCount[s.LibraryTrack.Artist]++
	}

	// Recent Artist should have at most 1 more (already had 1)
	if artistCount["Recent Artist"] > 1 {
		t.Errorf("Recent Artist count = %d, expected <= 1", artistCount["Recent Artist"])
	}
}

func TestSelectTracks_Empty(t *testing.T) {
	selected := selectTracks(nil, 5, nil, 10)
	if selected != nil {
		t.Errorf("expected nil for empty candidates, got %v", selected)
	}

	selected = selectTracks([]Candidate{}, 5, nil, 10)
	if selected != nil {
		t.Errorf("expected nil for empty candidates, got %v", selected)
	}
}

func TestSelectTracks_ZeroScores(t *testing.T) {
	// All zero scores should use uniform distribution
	candidates := []Candidate{
		{LibraryTrack: library.Track{Path: "/a.mp3", Artist: "Artist A"}, Score: 0},
		{LibraryTrack: library.Track{Path: "/b.mp3", Artist: "Artist B"}, Score: 0},
		{LibraryTrack: library.Track{Path: "/c.mp3", Artist: "Artist C"}, Score: 0},
	}

	selected := selectTracks(candidates, 2, nil, 10)

	if len(selected) != 2 {
		t.Errorf("expected 2 selected even with zero scores, got %d", len(selected))
	}
}

func TestSelectTracks_WeightedSelection(t *testing.T) {
	// High score track should be selected more often
	candidates := []Candidate{
		{LibraryTrack: library.Track{Path: "/high.mp3", Artist: "A"}, Score: 100.0},
		{LibraryTrack: library.Track{Path: "/low.mp3", Artist: "B"}, Score: 0.01},
	}

	highCount := 0
	iterations := 100

	for range iterations {
		selected := selectTracks(candidates, 1, nil, 10)
		if len(selected) == 1 && selected[0].LibraryTrack.Path == "/high.mp3" {
			highCount++
		}
	}

	// High score should be selected most of the time (at least 80%)
	if highCount < iterations*80/100 {
		t.Errorf("high score track selected %d/%d times, expected mostly", highCount, iterations)
	}
}

// Issue #269: each draw is made among the candidates that may still be
// selected, so selectTracks returns as many tracks as can be selected under
// the artist limit, candidates scored 0 last.

// selectRuns is how many times the tests of the draw run it: enough for the
// draw to show what it does at random.
const selectRuns = 1000

// candidate is a candidate by artist at path, scored score.
func candidate(path, artist string, score float64) Candidate {
	return Candidate{LibraryTrack: library.Track{Path: path, Artist: artist}, Score: score}
}

// candidatePaths returns the paths of candidates, in order.
func candidatePaths(candidates []Candidate) []string {
	p := make([]string, len(candidates))
	for i := range candidates {
		p[i] = candidates[i].LibraryTrack.Path
	}
	return p
}

func TestSelectTracks_SkipsTheTracksOfAnArtistAtTheLimit(t *testing.T) {
	candidates := make([]Candidate, 0, 20)
	for i := range 19 {
		candidates = append(candidates, candidate(fmt.Sprintf("/big/%d.mp3", i), "Big", 1))
	}
	candidates = append(candidates, candidate("/small.mp3", "Small", 1))

	for range selectRuns {
		selected := selectTracks(candidates, 1, map[string]int{"Big": 2}, 2)

		if got := candidatePaths(selected); !slices.Equal(got, []string{"/small.mp3"}) {
			t.Fatalf("selected %v, want /small.mp3: Big is at the limit", got)
		}
	}
}

func TestSelectTracks_FillsUpOnceAnArtistReachesTheLimit(t *testing.T) {
	others := []string{"/two.mp3", "/three.mp3", "/four.mp3"}
	candidates := make([]Candidate, 0, 30+len(others))
	for i := range 30 {
		candidates = append(candidates, candidate(fmt.Sprintf("/one/%d.mp3", i), "One", 1))
	}
	for i, p := range others {
		candidates = append(candidates, candidate(p, fmt.Sprintf("Other %d", i), 0.1))
	}

	for range selectRuns {
		selected := selectTracks(candidates, 5, nil, 2)

		if len(selected) != 5 {
			t.Fatalf("selected %v, want 5 tracks", candidatePaths(selected))
		}
		byOne := 0
		for i := range selected {
			if selected[i].LibraryTrack.Artist == "One" {
				byOne++
			}
		}
		for _, p := range others {
			if !slices.Contains(candidatePaths(selected), p) {
				t.Fatalf("selected %v, want %s among them", candidatePaths(selected), p)
			}
		}
		if byOne != 2 {
			t.Fatalf("selected %v, want 2 tracks by One", candidatePaths(selected))
		}
	}
}

// scoredAndZero are 3 candidates with a score and 3 scored 0, whose scores
// leave a rounding error once the first three are subtracted from their sum.
func scoredAndZero() (candidates []Candidate, scored, zero []string) {
	scored = []string{"/s1.mp3", "/s2.mp3", "/s3.mp3"}
	zero = []string{"/z1.mp3", "/z2.mp3", "/z3.mp3"}
	for i, score := range []float64{0.36, 0.0135, 0.009} {
		candidates = append(candidates, candidate(scored[i], fmt.Sprintf("S%d", i), score))
	}
	for i, p := range zero {
		candidates = append(candidates, candidate(p, fmt.Sprintf("Z%d", i), 0))
	}
	return candidates, scored, zero
}

func TestSelectTracks_PicksTheCandidatesScored0Last(t *testing.T) {
	candidates, scored, zero := scoredAndZero()
	picked := make(map[string]int)

	for range selectRuns {
		selected := candidatePaths(selectTracks(candidates, 5, nil, 10))

		if len(selected) != 5 {
			t.Fatalf("selected %v, want 5 tracks", selected)
		}
		for i, p := range selected {
			if want := i < 3; slices.Contains(scored, p) != want {
				t.Fatalf("selected %v, want the 3 scored tracks before the tracks scored 0", selected)
			}
		}
		for _, p := range selected[3:] {
			picked[p]++
		}
	}

	for _, p := range zero {
		if picked[p] == 0 {
			t.Errorf("%s picked in no run, want each track scored 0 picked in some: %v", p, picked)
		}
	}
}

func TestSelectTracks_PicksNoCandidateScored0WhileOneWithAScoreIsLeft(t *testing.T) {
	candidates, _, zero := scoredAndZero()

	for range selectRuns {
		selected := candidatePaths(selectTracks(candidates, 3, nil, 10))

		if len(selected) != 3 {
			t.Fatalf("selected %v, want 3 tracks", selected)
		}
		for _, p := range zero {
			if slices.Contains(selected, p) {
				t.Fatalf("selected %v, want no track scored 0", selected)
			}
		}
	}
}

func TestSelectTracks_PicksACandidateScored0OnceTheScoredOnesAreOverTheLimit(t *testing.T) {
	candidates := []Candidate{
		candidate("/big1.mp3", "Big", 1),
		candidate("/big2.mp3", "Big", 1),
		candidate("/big3.mp3", "Big", 1),
		candidate("/small.mp3", "Small", 0),
	}

	for range selectRuns {
		selected := candidatePaths(selectTracks(candidates, 3, nil, 2))

		if len(selected) != 3 || selected[2] != "/small.mp3" {
			t.Fatalf("selected %v, want 2 tracks by Big then /small.mp3", selected)
		}
	}
}

func TestSelectTracks_SelectsATrackInTheCandidatesTwiceOnce(t *testing.T) {
	// A local artist matched by two similar artists brings its tracks to
	// the candidates twice.
	candidates := []Candidate{
		candidate("/a.mp3", "A", 1),
		candidate("/a.mp3", "A", 1),
	}

	selected := candidatePaths(selectTracks(candidates, 2, nil, 10))

	if !slices.Equal(selected, []string{"/a.mp3"}) {
		t.Errorf("selected %v, want /a.mp3 once", selected)
	}
}

func TestSelectTracks_SelectsNothingWhenEveryArtistIsAtTheLimit(t *testing.T) {
	candidates := []Candidate{
		candidate("/big.mp3", "Big", 1),
		candidate("/small.mp3", "Small", 0),
	}

	selected := selectTracks(candidates, 2, map[string]int{"Big": 2, "Small": 2}, 2)

	if len(selected) != 0 {
		t.Errorf("selected %v, want nothing: every artist is at the limit", candidatePaths(selected))
	}
}

// Test selectArtistsWeighted

func TestSelectArtistsWeighted_Basic(t *testing.T) {
	artists := []MatchedArtist{
		{LocalArtist: "Artist A", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.9}},
		{LocalArtist: "Artist B", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.7}},
		{LocalArtist: "Artist C", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.5}},
	}

	selected := selectArtistsWeighted(artists, 2)

	if len(selected) != 2 {
		t.Errorf("expected 2 selected, got %d", len(selected))
	}
}

func TestSelectArtistsWeighted_Empty(t *testing.T) {
	selected := selectArtistsWeighted(nil, 5)
	if selected != nil {
		t.Errorf("expected nil for empty artists, got %v", selected)
	}

	selected = selectArtistsWeighted([]MatchedArtist{}, 5)
	if selected != nil {
		t.Errorf("expected nil for empty artists, got %v", selected)
	}
}

func TestSelectArtistsWeighted_RequestMoreThanAvailable(t *testing.T) {
	artists := []MatchedArtist{
		{LocalArtist: "Artist A", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.9}},
		{LocalArtist: "Artist B", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.7}},
	}

	selected := selectArtistsWeighted(artists, 5)

	if len(selected) != 2 {
		t.Errorf("expected 2 selected (all available), got %d", len(selected))
	}
}

func TestSelectArtistsWeighted_NoDuplicates(t *testing.T) {
	artists := []MatchedArtist{
		{LocalArtist: "Artist A", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.9}},
		{LocalArtist: "Artist B", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.8}},
		{LocalArtist: "Artist C", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.7}},
	}

	for range 50 {
		selected := selectArtistsWeighted(artists, 3)
		seen := make(map[string]bool)
		for _, s := range selected {
			if seen[s.LocalArtist] {
				t.Errorf("duplicate artist selected: %s", s.LocalArtist)
			}
			seen[s.LocalArtist] = true
		}
	}
}

func TestSelectArtistsWeighted_WeightedDistribution(t *testing.T) {
	// Artist with much higher score should be selected more often
	artists := []MatchedArtist{
		{LocalArtist: "High", LastfmArtist: lastfm.SimilarArtist{MatchScore: 1.0}},
		{LocalArtist: "Low", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.1}},
	}

	highCount := 0
	iterations := 100

	for range iterations {
		selected := selectArtistsWeighted(artists, 1)
		if len(selected) == 1 && selected[0].LocalArtist == "High" {
			highCount++
		}
	}

	// High score should be selected most of the time (at least 70%)
	// but not always (randomness should allow low-score artists sometimes)
	if highCount < iterations*70/100 {
		t.Errorf("high score artist selected %d/%d times, expected mostly", highCount, iterations)
	}
	if highCount == iterations {
		t.Logf("warning: high score artist selected every time, randomness may not be working")
	}
}

func TestSelectArtistsWeighted_MinimumWeight(t *testing.T) {
	// Artists with zero or very low scores should still have a chance (floor of 0.1)
	artists := []MatchedArtist{
		{LocalArtist: "Zero", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.0}},
		{LocalArtist: "Normal", LastfmArtist: lastfm.SimilarArtist{MatchScore: 0.5}},
	}

	zeroCount := 0
	iterations := 200

	for range iterations {
		selected := selectArtistsWeighted(artists, 1)
		if len(selected) == 1 && selected[0].LocalArtist == "Zero" {
			zeroCount++
		}
	}

	// Zero-score artist should be selected sometimes (due to minimum weight floor)
	if zeroCount == 0 {
		t.Error("zero-score artist was never selected, minimum weight floor may not be working")
	}
}

// What the radio does with a 0 for each config key that accepts it.

func TestCalculateScore_ZeroTopTrackBoostScoresATopTrackAsAnyOther(t *testing.T) {
	r := &Radio{config: config.RadioConfig{UserBoost: 1.3, FavoriteBoost: 2.0, DecayFactor: 0.1, MinSimilarityWeight: 0.1}}

	first := r.calculateScore(Candidate{GlobalPlaycount: 5000000, Rank: 1, SimilarityScore: 1.0})
	unranked := r.calculateScore(Candidate{GlobalPlaycount: 5000000, SimilarityScore: 1.0})

	if first != unranked {
		t.Errorf("rank 1 score = %v, want the unranked score %v", first, unranked)
	}
}

func TestCalculateScore_ZeroDecayFactorScoresARecentlyPlayedTrack0(t *testing.T) {
	r := &Radio{config: config.RadioConfig{TopTrackBoost: 3.0, UserBoost: 1.3, FavoriteBoost: 2.0, MinSimilarityWeight: 0.1}}

	score := r.calculateScore(Candidate{GlobalPlaycount: 5000000, RecentlyPlayed: true, SimilarityScore: 1.0})

	if score != 0 {
		t.Errorf("recently played score = %v, want 0", score)
	}
}

func TestCalculateScore_ZeroMinSimilarityWeightIsNoFloor(t *testing.T) {
	r := &Radio{config: config.RadioConfig{TopTrackBoost: 3.0, UserBoost: 1.3, FavoriteBoost: 2.0, DecayFactor: 0.1}}

	// base score 0.5 (5M plays), no boost, similarity 0.03 kept as it is
	score := r.calculateScore(Candidate{GlobalPlaycount: 5000000, SimilarityScore: 0.03})

	if score < 0.0149 || score > 0.0151 {
		t.Errorf("score = %v, want 0.015", score)
	}
}

func TestRadio_ZeroRecentSeedsWindowRemembersNoSeed(t *testing.T) {
	r := &Radio{config: config.RadioConfig{RecentSeedsWindow: 0}}

	r.AddRecentSeeds("Radiohead")
	r.AddRecentSeeds("Muse")

	if len(r.state.RecentSeeds) != 0 {
		t.Errorf("recent seeds = %v, want none", r.state.RecentSeeds)
	}
}

func TestMatchArtists_ZeroThresholdMatchesTheClosestArtistHoweverFar(t *testing.T) {
	similar := []lastfm.SimilarArtist{{Name: "Portishead", MatchScore: 0.9}}
	localArtists := []string{"Radiohead", "Abba"}

	if far := matchArtists(similar, localArtists, 0.8); len(far) != 0 {
		t.Fatalf("with threshold 0.8, matched %v, want nothing", far)
	}

	matched := matchArtists(similar, localArtists, 0)

	if len(matched) != 1 || matched[0].LocalArtist != "Radiohead" {
		t.Errorf("with threshold 0, matched %v, want Radiohead", matched)
	}
}
