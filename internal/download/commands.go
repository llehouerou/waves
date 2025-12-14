package download

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/slskd"
)

// Search timeout: 120 polls at 500ms each = 60 seconds max wait for search completion.
const maxSearchPolls = 120

// searchArtists searches for artists on MusicBrainz.
func searchArtists(client *musicbrainz.Client, query string) tea.Cmd {
	return func() tea.Msg {
		artists, err := client.SearchArtists(query)
		return ArtistSearchResultMsg{Artists: artists, Err: err}
	}
}

// fetchReleaseGroups fetches release groups for an artist.
func fetchReleaseGroups(client *musicbrainz.Client, artistID string) tea.Cmd {
	return func() tea.Msg {
		groups, err := client.GetArtistReleaseGroups(artistID)
		return ReleaseGroupResultMsg{ReleaseGroups: groups, Err: err}
	}
}

// fetchReleases fetches releases for a release group.
func fetchReleases(client *musicbrainz.Client, releaseGroupID string) tea.Cmd {
	return func() tea.Msg {
		releases, err := client.GetReleaseGroupReleases(releaseGroupID)
		return ReleaseResultMsg{Releases: releases, Err: err}
	}
}

// startSlskdSearch initiates a search on slskd.
func startSlskdSearch(client *slskd.Client, query string) tea.Cmd {
	return func() tea.Msg {
		searchID, err := client.Search(query)
		return SlskdSearchStartedMsg{SearchID: searchID, Err: err}
	}
}

// pollSlskdSearch polls for slskd search status and results.
// We fetch responses on every poll because they stream in over time.
// Returns a tea.Cmd that either completes with results or schedules the next poll after a delay.
func pollSlskdSearch(
	client *slskd.Client,
	searchID string,
	lastResponseCount int,
	stablePolls int,
	fetchRetries int,
	totalPolls int,
) tea.Cmd {
	return func() tea.Msg {
		// Check search status
		status, err := client.GetSearchStatus(searchID)
		if err != nil {
			return SlskdSearchResultMsg{Err: err}
		}

		state := slskd.SearchState(status.State)
		nextTotalPolls := totalPolls + 1

		// Check for timeout - if we've been polling too long, fetch whatever we have
		if nextTotalPolls >= maxSearchPolls && !state.IsComplete() {
			responses, fetchErr := client.GetSearchResponses(searchID)
			if fetchErr != nil {
				return SlskdSearchResultMsg{Err: fetchErr}
			}
			return SlskdSearchResultMsg{RawResponses: responses}
		}

		// Keep polling if search is still in progress
		if !state.IsComplete() {
			return SlskdPollContinueMsg{
				SearchID:      searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   0,
				FetchRetries:  0,
				TotalPolls:    nextTotalPolls,
			}
		}

		// Search is complete - check if responses are still coming in
		if status.ResponseCount > lastResponseCount {
			// Responses are still coming in, reset stable counter
			return SlskdPollContinueMsg{
				SearchID:      searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   0,
				FetchRetries:  0,
				TotalPolls:    nextTotalPolls,
			}
		}

		// Response count is stable - wait for 6 stable polls (~3 seconds) before first fetch attempt
		if stablePolls < 6 {
			return SlskdPollContinueMsg{
				SearchID:      searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   stablePolls + 1,
				FetchRetries:  0,
				TotalPolls:    nextTotalPolls,
			}
		}

		// Get search responses
		responses, err := client.GetSearchResponses(searchID)
		if err != nil {
			return SlskdSearchResultMsg{Err: err}
		}

		// If we have no responses but slskd reports some, keep retrying
		// Wait up to ~10 seconds (20 retries at 500ms each)
		if len(responses) == 0 && status.ResponseCount > 0 && fetchRetries < 20 {
			return SlskdPollContinueMsg{
				SearchID:      searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   stablePolls,
				FetchRetries:  fetchRetries + 1,
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

// queueDownload queues files for download on slskd.
func queueDownload(client *slskd.Client, result SlskdResult) tea.Cmd {
	return func() tea.Msg {
		err := client.Download(result.Username, result.Files)
		return SlskdDownloadQueuedMsg{Err: err}
	}
}

// scheduleSlskdPoll schedules the first poll with a delay.
func scheduleSlskdPoll(searchID string) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return SlskdSearchPollMsg{SearchID: searchID}
	})
}

// scheduleSlskdPollWithState schedules the next poll with preserved state and a delay.
func scheduleSlskdPollWithState(state SlskdPollContinueMsg) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return SlskdSearchPollMsg(state)
	})
}
