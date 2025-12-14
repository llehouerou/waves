package download

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/slskd"
)

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
func pollSlskdSearch(
	client *slskd.Client,
	searchID string,
	lastResponseCount int,
	stablePolls int,
	fetchRetries int,
) tea.Cmd {
	return func() tea.Msg {
		// Check search status
		status, err := client.GetSearchStatus(searchID)
		if err != nil {
			return SlskdSearchResultMsg{Err: err}
		}

		state := slskd.SearchState(status.State)

		// Keep polling if search is still in progress
		if !state.IsComplete() {
			return SlskdSearchPollMsg{
				SearchID:      searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   0,
				FetchRetries:  0,
			}
		}

		// Search is complete - check if responses are still coming in
		if status.ResponseCount > lastResponseCount {
			// Responses are still coming in, reset stable counter
			return SlskdSearchPollMsg{
				SearchID:      searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   0,
				FetchRetries:  0,
			}
		}

		// Response count is stable - wait for 6 stable polls (~3 seconds) before first fetch attempt
		if stablePolls < 6 {
			return SlskdSearchPollMsg{
				SearchID:      searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   stablePolls + 1,
				FetchRetries:  0,
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
			return SlskdSearchPollMsg{
				SearchID:      searchID,
				State:         status.State,
				ResponseCount: status.ResponseCount,
				StablePolls:   stablePolls,
				FetchRetries:  fetchRetries + 1,
			}
		}

		return SlskdSearchResultMsg{RawResponses: responses}
	}
}

// queueDownload queues files for download on slskd.
func queueDownload(client *slskd.Client, result SlskdResult) tea.Cmd {
	return func() tea.Msg {
		err := client.Download(result.Username, result.Files)
		return SlskdDownloadQueuedMsg{Err: err}
	}
}

// scheduleSlskdPoll schedules the next poll with a delay.
func scheduleSlskdPoll(searchID string) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return SlskdSearchPollMsg{SearchID: searchID}
	})
}
