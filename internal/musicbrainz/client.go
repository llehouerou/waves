package musicbrainz

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	baseURL      = "https://musicbrainz.org/ws/2"
	userAgent    = "Waves/0.1 (https://github.com/llehouerou/waves)"
	rateLimitDur = time.Second // MusicBrainz requires 1 request per second

	// Retry configuration
	maxRetries   = 3
	initialDelay = 2 * time.Second
	maxDelay     = 30 * time.Second
)

// Client provides access to the MusicBrainz API.
type Client struct {
	httpClient  *http.Client
	lastRequest time.Time
	mu          sync.Mutex
}

// NewClient creates a new MusicBrainz API client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SearchReleases searches for album releases matching the query.
// Query can be artist name, album name, or both.
func (c *Client) SearchReleases(query string) ([]Release, error) {
	c.waitForRateLimit()

	// Build search URL with album filter
	// Use MusicBrainz Lucene query syntax to filter by primary type
	// Wrap user query in parentheses for proper boolean logic
	params := url.Values{}
	params.Set("query", "("+query+") AND primarytype:album")
	params.Set("fmt", "json")
	params.Set("limit", "25")

	reqURL := fmt.Sprintf("%s/release?%s", baseURL, params.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API status %d: %s", resp.StatusCode, string(body))
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.convertReleases(result.Releases), nil
}

// GetRelease fetches detailed information about a specific release.
func (c *Client) GetRelease(mbid string) (*ReleaseDetails, error) {
	c.waitForRateLimit()

	// Include recordings (tracks) in the response
	params := url.Values{}
	params.Set("fmt", "json")
	params.Set("inc", "recordings+artist-credits")

	reqURL := fmt.Sprintf("%s/release/%s?%s", baseURL, mbid, params.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result releaseDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.convertReleaseDetails(result), nil
}

// waitForRateLimit ensures we don't exceed MusicBrainz rate limits.
func (c *Client) waitForRateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	elapsed := time.Since(c.lastRequest)
	if elapsed < rateLimitDur {
		time.Sleep(rateLimitDur - elapsed)
	}
	c.lastRequest = time.Now()
}

// doRequestWithRetry executes an HTTP request with exponential backoff retry.
// Retries on 5xx errors and network errors.
func (c *Client) doRequestWithRetry(req *http.Request) (*http.Response, error) {
	var lastErr error
	delay := initialDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
			delay = min(delay*2, maxDelay)
			c.waitForRateLimit() // Re-apply rate limit after retry delay
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Success or client error (4xx) - don't retry
		if resp.StatusCode < 500 {
			return resp, nil
		}

		// Server error (5xx) - retry
		resp.Body.Close()
		lastErr = fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries+1, lastErr)
}

// convertReleases converts raw API results to Release structs.
func (c *Client) convertReleases(results []releaseResult) []Release {
	releases := make([]Release, 0, len(results))

	for i := range results {
		r := &results[i]
		release := Release{
			ID:      r.ID,
			Title:   r.Title,
			Artist:  extractArtist(r.ArtistCredit),
			Date:    r.Date,
			Country: r.Country,
			Score:   r.Score,
		}

		if r.ReleaseGroup != nil {
			release.ReleaseType = r.ReleaseGroup.PrimaryType
		}

		// Sum track counts and collect formats
		var formats []string
		for _, m := range r.Media {
			release.TrackCount += m.TrackCount
			if m.Format != "" {
				formats = append(formats, m.Format)
			}
		}
		release.Formats = strings.Join(formats, ", ")

		releases = append(releases, release)
	}

	// Sort by release date descending (newest first)
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].Date > releases[j].Date
	})

	return releases
}

// convertReleaseDetails converts a raw release details response.
func (c *Client) convertReleaseDetails(r releaseDetailsResponse) *ReleaseDetails {
	details := &ReleaseDetails{
		Release: Release{
			ID:      r.ID,
			Title:   r.Title,
			Artist:  extractArtist(r.ArtistCredit),
			Date:    r.Date,
			Country: r.Country,
		},
	}

	if r.ReleaseGroup != nil {
		details.ReleaseType = r.ReleaseGroup.PrimaryType
	}

	// Collect all tracks from all media
	var formats []string
	for _, m := range r.Media {
		details.TrackCount += m.TrackCount
		if m.Format != "" {
			formats = append(formats, m.Format)
		}
		for _, t := range m.Tracks {
			details.Tracks = append(details.Tracks, Track(t))
		}
	}
	details.Formats = strings.Join(formats, ", ")

	return details
}

// extractArtist extracts the artist name from artist credits.
func extractArtist(credits []artistCredit) string {
	if len(credits) == 0 {
		return ""
	}

	parts := make([]string, 0, len(credits))
	for _, c := range credits {
		name := c.Name
		if name == "" {
			name = c.Artist.Name
		}
		parts = append(parts, name+c.JoinPhrase)
	}
	return strings.Join(parts, "")
}

// SearchArtists searches for artists matching the query.
func (c *Client) SearchArtists(query string) ([]Artist, error) {
	c.waitForRateLimit()

	params := url.Values{}
	params.Set("query", query)
	params.Set("fmt", "json")
	params.Set("limit", "25")

	reqURL := fmt.Sprintf("%s/artist?%s", baseURL, params.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API status %d: %s", resp.StatusCode, string(body))
	}

	var result artistSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.convertArtists(result.Artists), nil
}

// convertArtists converts raw API results to Artist structs.
func (c *Client) convertArtists(results []artistResult) []Artist {
	artists := make([]Artist, 0, len(results))
	for _, r := range results {
		a := Artist{
			ID:             r.ID,
			Name:           r.Name,
			SortName:       r.SortName,
			Type:           r.Type,
			Country:        r.Country,
			Score:          r.Score,
			Disambiguation: r.Disambiguation,
		}
		if r.LifeSpan != nil {
			a.BeginYear = extractYear(r.LifeSpan.Begin)
			a.EndYear = extractYear(r.LifeSpan.End)
		}
		artists = append(artists, a)
	}
	return artists
}

// extractYear returns the year portion of a date string (YYYY-MM-DD or YYYY).
func extractYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return date
}

// GetArtistReleaseGroups returns all release groups for an artist.
func (c *Client) GetArtistReleaseGroups(artistID string) ([]ReleaseGroup, error) {
	c.waitForRateLimit()

	params := url.Values{}
	params.Set("artist", artistID)
	params.Set("fmt", "json")
	params.Set("limit", "100")

	reqURL := fmt.Sprintf("%s/release-group?%s", baseURL, params.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API status %d: %s", resp.StatusCode, string(body))
	}

	var result releaseGroupBrowseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.convertReleaseGroups(result.ReleaseGroups), nil
}

// convertReleaseGroups converts raw API results to ReleaseGroup structs.
func (c *Client) convertReleaseGroups(results []releaseGroupResult) []ReleaseGroup {
	groups := make([]ReleaseGroup, 0, len(results))
	for _, r := range results {
		groups = append(groups, ReleaseGroup{
			ID:             r.ID,
			Title:          r.Title,
			PrimaryType:    r.PrimaryType,
			SecondaryTypes: r.SecondaryTypes,
			FirstRelease:   r.FirstRelease,
			Artist:         extractArtist(r.ArtistCredit),
		})
	}

	// Sort by release date descending (newest first)
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].FirstRelease > groups[j].FirstRelease
	})

	return groups
}

// GetReleaseGroupReleases returns all releases for a release group.
func (c *Client) GetReleaseGroupReleases(releaseGroupID string) ([]Release, error) {
	c.waitForRateLimit()

	params := url.Values{}
	params.Set("release-group", releaseGroupID)
	params.Set("fmt", "json")
	params.Set("inc", "media")
	params.Set("limit", "100")

	reqURL := fmt.Sprintf("%s/release?%s", baseURL, params.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.doRequestWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API status %d: %s", resp.StatusCode, string(body))
	}

	var result releaseBrowseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.convertReleases(result.Releases), nil
}
