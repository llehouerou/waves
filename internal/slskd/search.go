package slskd

import (
	"context"
	"time"
)

// How long RunSearch waits, counted in polls.
const (
	// searchMaxPolls is when a search slskd never finishes is read as it is.
	searchMaxPolls = 120
	// searchSettledPolls is how many polls must bring no new user before the
	// responses of a finished search are read: they keep coming for a while.
	searchSettledPolls = 6
	// searchFetchRetries is how many times the responses are read again when
	// slskd counts some but returns none yet.
	searchFetchRetries = 20
)

// The phases of a search, in order.
const (
	PhaseSearching  = "searching"  // slskd searches, or users still answer
	PhaseCollecting = "collecting" // no new user: waiting for the count to settle
	PhaseFetching   = "fetching"   // reading responses slskd does not return yet
)

// SearchProgress is where a running search is.
type SearchProgress struct {
	Phase     string // PhaseSearching, PhaseCollecting or PhaseFetching
	State     string // slskd's own state: "InProgress", "Completed, TimedOut"...
	Responses int    // users who have answered so far
	Elapsed   time.Duration
}

// RunSearch searches the Soulseek network for query and returns the responses
// once they have settled. It polls slskd every period, calling progress after
// each poll that leaves it waiting, and stops with ctx's error when ctx ends.
func (c *Client) RunSearch(
	ctx context.Context, query string, every time.Duration, progress func(SearchProgress),
) ([]SearchResponse, error) {
	id, err := c.Search(query)
	if err != nil {
		return nil, err
	}

	started := time.Now()
	var lastCount, settled, retries int
	for polls := 1; ; polls++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(every):
		}

		status, err := c.GetSearchStatus(id)
		if err != nil {
			return nil, err
		}
		switch finished := SearchState(status.State).IsComplete(); {
		case !finished && polls >= searchMaxPolls:
			return c.GetSearchResponses(id)
		case !finished, status.ResponseCount > lastCount:
			settled, retries = 0, 0
		case settled < searchSettledPolls:
			settled++
		default:
			responses, err := c.GetSearchResponses(id)
			if err != nil {
				return nil, err
			}
			if len(responses) > 0 || status.ResponseCount == 0 || retries >= searchFetchRetries {
				return responses, nil
			}
			retries++
		}
		lastCount = status.ResponseCount

		p := SearchProgress{
			Phase:     PhaseSearching,
			State:     status.State,
			Responses: status.ResponseCount,
			Elapsed:   time.Since(started),
		}
		switch {
		case retries > 0:
			p.Phase = PhaseFetching
		case settled > 0:
			p.Phase = PhaseCollecting
		}
		progress(p)
	}
}
