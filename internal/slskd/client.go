package slskd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client provides access to the slskd API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new slskd API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Search initiates a new search on the Soulseek network.
// Returns the search ID that can be used to poll for results.
func (c *Client) Search(query string) (string, error) {
	body := map[string]string{"searchText": query}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v0/searches", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result SearchRequest
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.ID, nil
}

// GetSearchStatus returns the current status of a search.
func (c *Client) GetSearchStatus(searchID string) (*SearchRequest, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v0/searches/"+searchID, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result SearchRequest
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// GetSearchResponses returns all responses for a search.
func (c *Client) GetSearchResponses(searchID string) ([]SearchResponse, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v0/searches/"+searchID+"/responses", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result []SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// Download queues files for download from a specific user.
func (c *Client) Download(username string, files []File) error {
	// Build the request - slskd expects files to be posted one at a time
	// or as a batch to the user's download endpoint
	for _, file := range files {
		body := map[string]any{
			"filename": file.Filename,
			"size":     file.Size,
		}
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}

		req, err := http.NewRequest(
			http.MethodPost,
			c.baseURL+"/api/v0/transfers/downloads/"+username,
			bytes.NewReader(jsonBody),
		)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("execute request: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API returned status %d for file %s", resp.StatusCode, file.Filename)
		}
	}

	return nil
}

// GetDownloads returns all current downloads.
func (c *Client) GetDownloads() ([]Download, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v0/transfers/downloads", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result []Download
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// DeleteSearch deletes a completed search.
func (c *Client) DeleteSearch(searchID string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/api/v0/searches/"+searchID, http.NoBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

// setHeaders sets common headers for API requests.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
}
