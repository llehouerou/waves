// Package radio implements Last.fm-based radio mode for automatic playlist generation.
package radio

import (
	"cmp"
	"database/sql"
	"math"
	"math/rand/v2"
	"slices"
	"sync"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/playlist"
)

// State holds the current radio mode state.
type State struct {
	Enabled        bool
	CurrentSeed    string   // Artist of last queued/playing track
	RecentlyPlayed []string // Track paths for decay scoring
}

// Radio manages the radio mode functionality.
type Radio struct {
	mu      sync.Mutex
	state   State
	config  config.RadioConfig
	client  *lastfm.Client
	library *library.Library
	cache   *Cache
	db      *sql.DB
}

// New creates a new Radio instance.
func New(db *sql.DB, client *lastfm.Client, lib *library.Library, cfg config.RadioConfig) *Radio {
	return &Radio{
		state:   State{},
		config:  cfg,
		client:  client,
		library: lib,
		cache:   NewCache(db, cfg.CacheTTLDays),
		db:      db,
	}
}

// Toggle enables or disables radio mode.
func (r *Radio) Toggle() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.state.Enabled = !r.state.Enabled
	if !r.state.Enabled {
		// Clear decay list when disabling
		r.state.RecentlyPlayed = nil
	}
	return r.state.Enabled
}

// Enable enables radio mode.
func (r *Radio) Enable() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.Enabled = true
}

// Disable disables radio mode.
func (r *Radio) Disable() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.Enabled = false
	r.state.RecentlyPlayed = nil
}

// IsEnabled returns true if radio mode is enabled.
func (r *Radio) IsEnabled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state.Enabled
}

// SetSeed sets the current seed artist.
func (r *Radio) SetSeed(artist string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.CurrentSeed = artist
}

// CurrentSeed returns the current seed artist.
func (r *Radio) CurrentSeed() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state.CurrentSeed
}

// AddToRecentlyPlayed adds a track path to the recently played list.
func (r *Radio) AddToRecentlyPlayed(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.RecentlyPlayed = append(r.state.RecentlyPlayed, path)
}

// IsRecentlyPlayed checks if a track path is in the recently played list.
func (r *Radio) IsRecentlyPlayed(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Contains(r.state.RecentlyPlayed, path)
}

// FillResult contains the result of filling the queue with radio tracks.
type FillResult struct {
	Tracks  []playlist.Track
	Message string // Transient message to display (e.g., "No related tracks found")
	Err     error
}

// Fill generates tracks to add to the queue based on the current seed.
// If no matches found for the seed, tries previously played artists as fallback.
// Returns up to bufferSize tracks.
func (r *Radio) Fill(seedArtist string) FillResult {
	r.mu.Lock()
	cfg := r.config
	recentlyPlayed := make([]string, len(r.state.RecentlyPlayed))
	copy(recentlyPlayed, r.state.RecentlyPlayed)
	r.mu.Unlock()

	if seedArtist == "" {
		return FillResult{Message: "No seed artist"}
	}

	// Get local artists from library (needed for all attempts)
	localArtists, err := r.library.Artists()
	if err != nil {
		return FillResult{Err: err}
	}

	// Try primary seed first
	if result := r.tryFillFromSeed(seedArtist, localArtists, recentlyPlayed, cfg); result != nil {
		return *result
	}

	// No matches for primary seed - try recently played artists as fallback
	triedArtists := map[string]bool{seedArtist: true}
	for i := len(recentlyPlayed) - 1; i >= 0; i-- {
		// Get artist from track path
		track, err := r.library.TrackByPath(recentlyPlayed[i])
		if err != nil || track == nil {
			continue
		}
		artist := track.Artist
		if artist == "" || triedArtists[artist] {
			continue
		}
		triedArtists[artist] = true

		if result := r.tryFillFromSeed(artist, localArtists, recentlyPlayed, cfg); result != nil {
			return *result
		}
	}

	return FillResult{Message: "No related tracks found"}
}

// tryFillFromSeed attempts to fill from a specific seed artist.
// Returns nil if no matches found, allowing caller to try another seed.
func (r *Radio) tryFillFromSeed(seedArtist string, localArtists, recentlyPlayed []string, cfg config.RadioConfig) *FillResult {
	// Get similar artists (from cache or API)
	similarArtists, err := r.getSimilarArtists(seedArtist)
	if err != nil {
		return &FillResult{Err: err}
	}

	if len(similarArtists) == 0 {
		return nil // Try next seed
	}

	// Match similar artists to local library using fuzzy matching
	matchedArtists := matchArtists(similarArtists, localArtists, cfg.ArtistMatchThreshold)

	if len(matchedArtists) == 0 {
		return nil // Try next seed
	}

	// Build candidate pool from matched artists
	candidates := r.buildCandidatePool(matchedArtists, recentlyPlayed)

	if len(candidates) == 0 {
		return nil // Try next seed
	}

	// Select tracks using weighted random
	selected := selectTracks(candidates, cfg.BufferSize)

	// Convert to playlist tracks
	tracks := make([]playlist.Track, 0, len(selected))
	for i := range selected {
		tracks = append(tracks, playlist.Track{
			ID:          selected[i].LibraryTrack.ID,
			Path:        selected[i].LibraryTrack.Path,
			Title:       selected[i].LibraryTrack.Title,
			Artist:      selected[i].LibraryTrack.Artist,
			Album:       selected[i].LibraryTrack.Album,
			TrackNumber: selected[i].LibraryTrack.TrackNumber,
		})
	}

	return &FillResult{Tracks: tracks}
}

// getSimilarArtists returns similar artists from cache or fetches from API.
func (r *Radio) getSimilarArtists(artist string) ([]lastfm.SimilarArtist, error) {
	// Try cache first
	cached, err := r.cache.GetSimilarArtists(artist)
	if err == nil && len(cached) > 0 {
		return cached, nil
	}

	// Fetch from API
	if r.client == nil {
		return nil, nil
	}

	similar, err := r.client.GetSimilarArtists(artist, 50)
	if err != nil {
		return nil, err
	}

	// Cache the results
	if len(similar) > 0 {
		_ = r.cache.SetSimilarArtists(artist, similar)
	}

	return similar, nil
}

// MatchedArtist pairs a Last.fm similar artist with a local library artist.
type MatchedArtist struct {
	LastfmArtist lastfm.SimilarArtist
	LocalArtist  string
}

// artistData holds fetched Last.fm data for an artist.
type artistData struct {
	artist     MatchedArtist
	topTracks  []lastfm.TopTrack
	userTracks []lastfm.UserTrack
}

// buildCandidatePool creates a pool of candidate tracks from matched artists.
// Uses weighted shuffle to select up to 5 artists, favoring higher match scores.
func (r *Radio) buildCandidatePool(matchedArtists []MatchedArtist, recentlyPlayed []string) []Candidate {
	// Weighted shuffle: higher match scores have better chance of being selected
	shuffled := weightedShuffleArtists(matchedArtists)

	// Limit to top 5 artists for performance
	maxArtists := min(5, len(shuffled))
	artists := shuffled[:maxArtists]

	// Fetch Last.fm data for all artists concurrently
	artistDataList := r.fetchArtistDataConcurrently(artists)

	// Build candidates from fetched data
	var candidates []Candidate
	for _, ad := range artistDataList {
		// Get all tracks for this artist from library
		libraryTracks, err := r.library.ArtistTracks(ad.artist.LocalArtist)
		if err != nil {
			continue
		}

		// Build lookup maps for fast matching
		topTrackMap := buildTopTrackMap(ad.topTracks)
		userTrackMap := buildUserTrackMap(ad.userTracks)

		// Create candidates
		for i := range libraryTracks {
			lt := &libraryTracks[i]
			candidate := Candidate{
				LibraryTrack:    *lt,
				SimilarityScore: ad.artist.LastfmArtist.MatchScore,
			}

			// Check top tracks for global playcount
			normTitle := normalizeString(lt.Title)
			if tt, ok := topTrackMap[normTitle]; ok {
				candidate.GlobalPlaycount = tt.Playcount
				candidate.Rank = tt.Rank
			}

			// Check user scrobbles
			if ut, ok := userTrackMap[normTitle]; ok {
				candidate.UserScrobbled = true
				candidate.UserPlaycount = ut.Playcount
			}

			// Check if recently played
			candidate.RecentlyPlayed = slices.Contains(recentlyPlayed, lt.Path)

			// Calculate final score
			candidate.Score = r.calculateScore(candidate)

			candidates = append(candidates, candidate)
		}
	}

	return candidates
}

// fetchArtistDataConcurrently fetches top tracks and user scrobbles for all artists in parallel.
func (r *Radio) fetchArtistDataConcurrently(artists []MatchedArtist) []artistData {
	results := make([]artistData, len(artists))
	var wg sync.WaitGroup

	for i, ma := range artists {
		wg.Add(1)
		go func(idx int, artist MatchedArtist) {
			defer wg.Done()

			ad := artistData{artist: artist}

			// Fetch top tracks (from cache or API)
			if topTracks, err := r.getArtistTopTracks(artist.LocalArtist); err == nil {
				ad.topTracks = topTracks
			}

			// Fetch user scrobbles (from cache or API)
			if userTracks, err := r.getUserArtistTracks(artist.LocalArtist); err == nil {
				ad.userTracks = userTracks
			}

			results[idx] = ad
		}(i, ma)
	}

	wg.Wait()
	return results
}

// buildTopTrackMap creates a normalized name -> TopTrack lookup map.
func buildTopTrackMap(tracks []lastfm.TopTrack) map[string]lastfm.TopTrack {
	m := make(map[string]lastfm.TopTrack, len(tracks))
	for _, t := range tracks {
		m[normalizeString(t.Name)] = t
	}
	return m
}

// buildUserTrackMap creates a normalized name -> UserTrack lookup map.
func buildUserTrackMap(tracks []lastfm.UserTrack) map[string]lastfm.UserTrack {
	m := make(map[string]lastfm.UserTrack, len(tracks))
	for _, t := range tracks {
		m[normalizeString(t.Name)] = t
	}
	return m
}

// weightedShuffleArtists shuffles artists with probability weighted by match score.
// Uses the Efraimidis-Spirakis algorithm for weighted random sampling.
// Higher match scores have a better chance of ranking first, but randomness adds variety.
func weightedShuffleArtists(artists []MatchedArtist) []MatchedArtist {
	if len(artists) <= 1 {
		return artists
	}

	type weighted struct {
		artist MatchedArtist
		key    float64
	}

	items := make([]weighted, len(artists))
	for i, a := range artists {
		score := a.LastfmArtist.MatchScore
		if score <= 0 {
			score = 0.01 // Minimum weight to avoid division by zero
		}
		// Efraimidis-Spirakis: key = -log(rand) / weight
		// Lower key = higher priority
		items[i] = weighted{
			artist: a,
			key:    -math.Log(rand.Float64()) / score, //nolint:gosec // not security-sensitive
		}
	}

	// Sort ascending by key (lower = higher priority)
	slices.SortFunc(items, func(a, b weighted) int {
		return cmp.Compare(a.key, b.key)
	})

	result := make([]MatchedArtist, len(artists))
	for i, item := range items {
		result[i] = item.artist
	}
	return result
}

// getArtistTopTracks returns top tracks from cache or fetches from API.
func (r *Radio) getArtistTopTracks(artist string) ([]lastfm.TopTrack, error) {
	// Try cache first
	cached, err := r.cache.GetArtistTopTracks(artist)
	if err == nil && len(cached) > 0 {
		return cached, nil
	}

	// Fetch from API
	if r.client == nil {
		return nil, nil
	}

	tracks, err := r.client.GetArtistTopTracks(artist, 50)
	if err != nil {
		return nil, err
	}

	// Cache the results
	if len(tracks) > 0 {
		_ = r.cache.SetArtistTopTracks(artist, tracks)
	}

	return tracks, nil
}

// getUserArtistTracks returns user scrobbles from cache or fetches from API.
func (r *Radio) getUserArtistTracks(artist string) ([]lastfm.UserTrack, error) {
	// Try cache first
	cached, err := r.cache.GetUserArtistTracks(artist)
	if err == nil && len(cached) > 0 {
		return cached, nil
	}

	// Fetch from API
	if r.client == nil || !r.client.IsAuthenticated() {
		return nil, nil
	}

	tracks, err := r.client.GetUserArtistTracks(artist, 200)
	if err != nil {
		return nil, err
	}

	// Cache the results
	if len(tracks) > 0 {
		_ = r.cache.SetUserArtistTracks(artist, tracks)
	}

	return tracks, nil
}

// calculateScore computes the final score for a candidate track.
func (r *Radio) calculateScore(c Candidate) float64 {
	// Base score from global popularity (normalized, assuming max ~10M plays)
	baseScore := float64(c.GlobalPlaycount) / 10000000.0
	if baseScore > 1.0 {
		baseScore = 1.0
	}
	if baseScore < 0.01 {
		baseScore = 0.01 // Minimum for tracks not in top 50
	}

	// User boost for scrobbled tracks
	userBoost := 1.0
	if c.UserScrobbled {
		userBoost = r.config.UserBoost
	}

	// Decay penalty for recently played
	decayPenalty := 1.0
	if c.RecentlyPlayed {
		decayPenalty = r.config.DecayFactor
	}

	// Similarity weight from Last.fm
	similarityWeight := c.SimilarityScore
	if similarityWeight < 0.1 {
		similarityWeight = 0.1
	}

	return baseScore * userBoost * decayPenalty * similarityWeight
}
