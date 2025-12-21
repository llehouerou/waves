//nolint:goconst // test files commonly repeat strings for test data
package radio

import (
	"testing"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/library"
)

// Test normalizeString

func TestNormalizeString_Basic(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"The Beatles", "the beatles"},
		{"AC/DC", "acdc"}, // slash is removed, not converted to space
		{"Guns N' Roses", "guns n roses"},
		{"P!nk", "pnk"},
		{"Ke$ha", "keha"},
		{"  Multiple   Spaces  ", "multiple spaces"},
		{"Under_Score", "under score"},
		{"Hyphen-Ated", "hyphen ated"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeString(tt.input)
			if got != tt.want {
				t.Errorf("normalizeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeString_RemasteredSuffixes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Song Title (Remastered)", "song title"},
		{"Song Title (Remaster)", "song title"},
		{"Song Title - Remastered", "song title"},
		{"Song Title [Remastered]", "song title"},
		{"Song Title (2023 Remaster)", "song title 2023 remaster"}, // Only specific patterns removed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeString(tt.input)
			if got != tt.want {
				t.Errorf("normalizeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeString_Unicode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Björk", "björk"},
		{"Sigur Rós", "sigur rós"},
		{"Café Tacvba", "café tacvba"},
		{"日本語", "日本語"}, // Japanese characters preserved
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeString(tt.input)
			if got != tt.want {
				t.Errorf("normalizeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

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

// Test buildTopTrackMap and buildUserTrackMap

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

func TestBuildUserTrackMap(t *testing.T) {
	tracks := []lastfm.UserTrack{
		{Name: "Creep", Playcount: 50},
		{Name: "Karma Police", Playcount: 30},
	}

	m := buildUserTrackMap(tracks)

	if ut, ok := m["creep"]; !ok || ut.Playcount != 50 {
		t.Error("expected to find 'creep' with playcount 50")
	}
	if ut, ok := m["karma police"]; !ok || ut.Playcount != 30 {
		t.Error("expected to find 'karma police' with playcount 30")
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
