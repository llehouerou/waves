package download

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/slskd"
)

// Search timeout: 120 polls at 500ms each = 60 seconds max wait for search completion.
const maxSearchPolls = 120

// startSlskdSearchCmd initiates a search on slskd.
func startSlskdSearchCmd(client *slskd.Client, query string) tea.Cmd {
	return func() tea.Msg {
		searchID, err := client.Search(query)
		return SlskdSearchStartedMsg{SearchID: searchID, Err: err}
	}
}

// slskdPollParams contains parameters for polling slskd search status.
type slskdPollParams struct {
	client            *slskd.Client
	searchID          string
	lastResponseCount int
	stablePolls       int
	fetchRetries      int
	totalPolls        int
}

// pollSlskdSearchCmd polls for slskd search status and results.
// We fetch responses on every poll because they stream in over time.
// Returns a tea.Cmd that either completes with results or schedules the next poll after a delay.
func pollSlskdSearchCmd(params slskdPollParams) tea.Cmd {
	return func() tea.Msg {
		// Check search status
		status, err := params.client.GetSearchStatus(params.searchID)
		if err != nil {
			return SlskdSearchResultMsg{Err: err}
		}

		state := slskd.SearchState(status.State)
		nextTotalPolls := params.totalPolls + 1

		// Check for timeout - if we've been polling too long, fetch whatever we have
		if nextTotalPolls >= maxSearchPolls && !state.IsComplete() {
			responses, fetchErr := params.client.GetSearchResponses(params.searchID)
			if fetchErr != nil {
				return SlskdSearchResultMsg{Err: fetchErr}
			}
			return SlskdSearchResultMsg{RawResponses: responses}
		}

		// Keep polling if search is still in progress
		if !state.IsComplete() {
			return SlskdPollContinueMsg{
				SearchID:      params.searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   0,
				FetchRetries:  0,
				TotalPolls:    nextTotalPolls,
			}
		}

		// Search is complete - check if responses are still coming in
		if status.ResponseCount > params.lastResponseCount {
			// Responses are still coming in, reset stable counter
			return SlskdPollContinueMsg{
				SearchID:      params.searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   0,
				FetchRetries:  0,
				TotalPolls:    nextTotalPolls,
			}
		}

		// Response count is stable - wait for 6 stable polls (~3 seconds) before first fetch attempt
		if params.stablePolls < 6 {
			return SlskdPollContinueMsg{
				SearchID:      params.searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   params.stablePolls + 1,
				FetchRetries:  0,
				TotalPolls:    nextTotalPolls,
			}
		}

		// Get search responses
		responses, err := params.client.GetSearchResponses(params.searchID)
		if err != nil {
			return SlskdSearchResultMsg{Err: err}
		}

		// If we have no responses but slskd reports some, keep retrying
		// Wait up to ~10 seconds (20 retries at 500ms each)
		if len(responses) == 0 && status.ResponseCount > 0 && params.fetchRetries < 20 {
			return SlskdPollContinueMsg{
				SearchID:      params.searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   params.stablePolls,
				FetchRetries:  params.fetchRetries + 1,
				TotalPolls:    nextTotalPolls,
			}
		}

		return SlskdSearchResultMsg{RawResponses: responses}
	}
}

// SlskdPollContinueMsg indicates polling should continue after a delay.
// Exported so app can route it to the download popup.
type SlskdPollContinueMsg struct {
	SearchID      string
	State         string
	ResponseCount int
	StablePolls   int
	FetchRetries  int
	TotalPolls    int
}

// queueDownloadCmd queues files for download on slskd.
func queueDownloadCmd(client *slskd.Client, result SlskdResult) tea.Cmd {
	return func() tea.Msg {
		err := client.Download(result.Username, result.Files)
		return SlskdDownloadQueuedMsg{Err: err}
	}
}

// scheduleSlskdPollCmd schedules the first poll with a delay.
func scheduleSlskdPollCmd(searchID string) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return SlskdSearchPollMsg{SearchID: searchID}
	})
}

// scheduleSlskdPollWithStateCmd schedules the next poll with preserved state and a delay.
func scheduleSlskdPollWithStateCmd(state SlskdPollContinueMsg) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return SlskdSearchPollMsg(state)
	})
}
