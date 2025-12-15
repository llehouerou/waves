package musicbrainz

import (
	"fmt"
	"io"
	"net/http"
)

const (
	coverArtBaseURL = "https://coverartarchive.org"
)

// GetCoverArt fetches the front cover for a release from Cover Art Archive.
// Returns the image data as bytes, or nil if no cover art is available.
// The image is fetched at 500px size for a good balance of quality and size.
func (c *Client) GetCoverArt(releaseMBID string) ([]byte, error) {
	c.waitForRateLimit()

	// Request the front cover at 500px size
	reqURL := fmt.Sprintf("%s/release/%s/front-500", coverArtBaseURL, releaseMBID)

	req, err := http.NewRequest(http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// 404 means no cover art available - not an error
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	// Handle redirects (307) - the client should follow them automatically
	// but check for other error codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Read the image data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return data, nil
}

// GetCoverArtLarge fetches the front cover at full resolution (1200px).
// Returns nil if no cover art is available.
func (c *Client) GetCoverArtLarge(releaseMBID string) ([]byte, error) {
	c.waitForRateLimit()

	reqURL := fmt.Sprintf("%s/release/%s/front-1200", coverArtBaseURL, releaseMBID)

	req, err := http.NewRequest(http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return data, nil
}
