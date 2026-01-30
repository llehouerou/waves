// Package lrclib provides a client for the lrclib.net lyrics API.
package lrclib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ErrNotFound is returned when no lyrics are found.
var ErrNotFound = errors.New("lyrics not found")

const (
	baseURL   = "https://lrclib.net/api"
	userAgent = "waves-music-player/1.0 (https://github.com/llehouerou/waves)"
)

// Client is an lrclib.net API client.
type Client struct {
	httpClient *http.Client
}

// New creates a new lrclib client.
func New() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// LyricsResult represents the response from the lrclib API.
type LyricsResult struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// Get fetches lyrics by artist, title, and optionally album and duration.
// Duration should be in seconds.
func (c *Client) Get(ctx context.Context, artist, title string, duration time.Duration) (*LyricsResult, error) {
	params := url.Values{}
	params.Set("artist_name", artist)
	params.Set("track_name", title)
	if duration > 0 {
		params.Set("duration", fmt.Sprintf("%.0f", duration.Seconds()))
	}

	reqURL := fmt.Sprintf("%s/get?%s", baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var result LyricsResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// Search searches for lyrics matching the query.
func (c *Client) Search(ctx context.Context, query string) ([]LyricsResult, error) {
	params := url.Values{}
	params.Set("q", query)

	reqURL := fmt.Sprintf("%s/search?%s", baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var results []LyricsResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return results, nil
}

// HasSyncedLyrics returns true if the result contains synced (LRC) lyrics.
func (r *LyricsResult) HasSyncedLyrics() bool {
	return r.SyncedLyrics != ""
}

// HasPlainLyrics returns true if the result contains plain text lyrics.
func (r *LyricsResult) HasPlainLyrics() bool {
	return r.PlainLyrics != ""
}
