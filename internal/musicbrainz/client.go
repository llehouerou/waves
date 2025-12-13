package musicbrainz

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	baseURL   = "https://musicbrainz.org/ws/2"
	userAgent = "Waves/0.1 (https://github.com/llehouerou/waves)"
	rateLimit = time.Second // MusicBrainz requires 1 request per second
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

// SearchReleases searches for releases matching the query.
// Query can be artist name, album name, or both.
func (c *Client) SearchReleases(query string) ([]Release, error) {
	c.rateLimit()

	// Build search URL
	params := url.Values{}
	params.Set("query", query)
	params.Set("fmt", "json")
	params.Set("limit", "25")

	reqURL := fmt.Sprintf("%s/release?%s", baseURL, params.Encode())

	req, err := http.NewRequest(http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return c.convertReleases(result.Releases), nil
}

// GetRelease fetches detailed information about a specific release.
func (c *Client) GetRelease(mbid string) (*ReleaseDetails, error) {
	c.rateLimit()

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

	resp, err := c.httpClient.Do(req)
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

// rateLimit ensures we don't exceed MusicBrainz rate limits.
func (c *Client) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	elapsed := time.Since(c.lastRequest)
	if elapsed < rateLimit {
		time.Sleep(rateLimit - elapsed)
	}
	c.lastRequest = time.Now()
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
