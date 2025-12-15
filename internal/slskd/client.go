package slskd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	// Use the /responses endpoint to get actual file responses
	reqURL := c.baseURL + "/api/v0/searches/" + searchID + "/responses"
	req, err := http.NewRequest(http.MethodGet, reqURL, http.NoBody)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result []SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}

// Download queues files for download from a specific user.
func (c *Client) Download(username string, files []File) error {
	// slskd expects an array of file objects
	jsonBody, err := json.Marshal(files)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// URL-encode the username
	encodedUsername := url.PathEscape(username)

	req, err := http.NewRequest(
		http.MethodPost,
		c.baseURL+"/api/v0/transfers/downloads/"+encodedUsername,
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetDownloads returns all current downloads as a flattened list.
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

	var responses []DownloadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&responses); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Flatten the nested structure into a list of downloads
	var downloads []Download
	for _, userResp := range responses {
		for _, dir := range userResp.Directories {
			for _, file := range dir.Files {
				downloads = append(downloads, Download{
					ID:               file.ID,
					Username:         file.Username,
					Filename:         file.Filename,
					State:            file.State,
					Size:             file.Size,
					BytesTransferred: file.BytesTransferred,
				})
			}
		}
	}

	return downloads, nil
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

// CancelDownload cancels/removes a download by its ID from a specific user.
func (c *Client) CancelDownload(username, downloadID string) error {
	encodedUsername := url.PathEscape(username)
	encodedID := url.PathEscape(downloadID)

	req, err := http.NewRequest(
		http.MethodDelete,
		c.baseURL+"/api/v0/transfers/downloads/"+encodedUsername+"/"+encodedID,
		http.NoBody,
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

	// Accept various success codes
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

// CancelDownloads cancels/removes multiple downloads for a user.
func (c *Client) CancelDownloads(username string, downloadIDs []string) error {
	for _, id := range downloadIDs {
		if err := c.CancelDownload(username, id); err != nil {
			// Log but continue - some might already be removed
			continue
		}
	}
	return nil
}

// setHeaders sets common headers for API requests.
func (c *Client) setHeaders(req *http.Request) {
	// Only set Content-Type for requests with a body (POST, PUT, PATCH)
	if req.Method == http.MethodPost || req.Method == http.MethodPut || req.Method == http.MethodPatch {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
}
