// Package slskd provides a client for the slskd API.
package slskd

import "strings"

// SearchRequest represents a search request to slskd.
type SearchRequest struct {
	ID            string `json:"id"`
	SearchText    string `json:"searchText"`
	Token         int    `json:"token"`
	State         string `json:"state"` // InProgress, Completed, etc.
	ResponseCount int    `json:"responseCount"`
}

// SearchResponse represents a user's response to a search.
type SearchResponse struct {
	Username    string `json:"username"`
	FileCount   int    `json:"fileCount"`
	HasFreeSlot bool   `json:"hasFreeUploadSlot"`
	QueueLength int    `json:"queueLength"`
	UploadSpeed int    `json:"uploadSpeed"` // bytes per second
	Files       []File `json:"files"`
}

// File represents a file in search results.
type File struct {
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	Code      int    `json:"code"`
	Extension string `json:"extension"`
	BitRate   int    `json:"bitRate"`
	BitDepth  int    `json:"bitDepth"`
	Length    int    `json:"length"` // Duration in seconds
	IsLocked  bool   `json:"isLocked"`
}

// DownloadRequest represents a request to download files.
type DownloadRequest struct {
	Username string `json:"username"`
	Files    []File `json:"files"`
}

// Download represents an active or completed download.
type Download struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Filename  string `json:"filename"`
	State     string `json:"state"` // Queued, InProgress, Completed, Errored
	Size      int64  `json:"size"`
	BytesRead int64  `json:"bytesRead"`
}

// SearchState represents the state of a search.
type SearchState string

const (
	SearchStateNone       SearchState = "None"
	SearchStateRequested  SearchState = "Requested"
	SearchStateInProgress SearchState = "InProgress"
	SearchStateCompleted  SearchState = "Completed"
	SearchStateTimedOut   SearchState = "TimedOut"
	SearchStateCancelled  SearchState = "Cancelled"
	SearchStateErrored    SearchState = "Errored"
)

// IsComplete returns true if the search is in a terminal state.
// States can be compound (e.g., "Completed, ResponseLimitReached").
func (s SearchState) IsComplete() bool {
	state := string(s)
	return strings.Contains(state, "Completed") ||
		strings.Contains(state, "TimedOut") ||
		strings.Contains(state, "Cancelled") ||
		strings.Contains(state, "Errored")
}
