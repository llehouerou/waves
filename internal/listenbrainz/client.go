// Package listenbrainz provides a client for the ListenBrainz API.
package listenbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/llehouerou/waves/internal/version"
)

const defaultBaseURL = "https://api.listenbrainz.org/1"

// WindowDays is the fresh-releases window, past and future. 90 is the hard
// server-side cap: beyond it the API answers HTTP 400, it never clamps.
const WindowDays = 90

// Release is one entry of the fresh-releases payload.
// PrimaryType and SecondaryType are absent from the JSON when null:
// an empty string means unknown type, not an error.
type Release struct {
	ReleaseGroupMBID string `json:"release_group_mbid"`
	ArtistCreditName string `json:"artist_credit_name"`
	ReleaseName      string `json:"release_name"`
	ReleaseDate      string `json:"release_date"`
	PrimaryType      string `json:"release_group_primary_type"`
	SecondaryType    string `json:"release_group_secondary_type"`
	ListenCount      int    `json:"listen_count"`
}

// Client is a ListenBrainz API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// New creates a new ListenBrainz client.
func New() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
	}
}

// FreshReleases fetches the worldwide fresh releases over the WindowDays window,
// past and upcoming.
func (c *Client) FreshReleases(ctx context.Context) ([]Release, error) {
	params := url.Values{}
	params.Set("days", strconv.Itoa(WindowDays))
	params.Set("past", "true")
	params.Set("future", "true")

	reqURL := fmt.Sprintf("%s/explore/fresh-releases/?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", version.UserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("listenbrainz status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Payload struct {
			Releases []Release `json:"releases"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Payload.Releases, nil
}
