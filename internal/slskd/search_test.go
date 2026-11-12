package slskd_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/slskd"
	"github.com/llehouerou/waves/internal/slskd/slskdtest"
)

// every is the poll period of the searches in these tests.
const every = time.Millisecond

// user is the response of one user holding one file.
func user(name string) slskd.SearchResponse {
	return slskd.SearchResponse{Username: name, Files: []slskd.File{{Filename: `@@` + name + `\Album\01.flac`}}}
}

func usernames(responses []slskd.SearchResponse) []string {
	names := make([]string, len(responses))
	for i := range responses {
		names[i] = responses[i].Username
	}
	return names
}

func TestRunSearch_WaitsForTheResponsesToSettle(t *testing.T) {
	fake := slskdtest.New(t)
	var phases []string
	progress := func(p slskd.SearchProgress) {
		phases = append(phases, p.Phase)
		// slskd finishes once the search has been seen in progress; a last
		// user answers after that.
		switch len(phases) {
		case 1:
			fake.SetSearch("Completed", 1, user("bob"))
		case 3:
			fake.SetSearch("Completed", 2, user("bob"), user("alice"))
		}
	}

	responses, err := fake.Client().RunSearch(context.Background(), "low trust", every, progress)
	if err != nil {
		t.Fatalf("RunSearch: %v", err)
	}

	if got := usernames(responses); !slices.Equal(got, []string{"bob", "alice"}) {
		t.Errorf("responses from %v, want bob and alice", got)
	}
	if got := fake.Searches(); !slices.Equal(got, []string{"low trust"}) {
		t.Errorf("searched %v, want low trust once", got)
	}
	// In progress, then finished with a first user, then the second user
	// starts the wait again: six polls with nothing new before reading.
	want := slices.Concat(
		[]string{slskd.PhaseSearching, slskd.PhaseSearching, slskd.PhaseCollecting, slskd.PhaseSearching},
		slices.Repeat([]string{slskd.PhaseCollecting}, 6),
	)
	if !slices.Equal(phases, want) {
		t.Errorf("phases = %v\nwant     %v", phases, want)
	}
}

// slskd counts a user before it can return what that user answered.
func TestRunSearch_ReadsAgainTheResponsesSlskdDoesNotReturnYet(t *testing.T) {
	fake := slskdtest.New(t)
	fake.SetSearch("Completed", 1)
	fetching := 0
	progress := func(p slskd.SearchProgress) {
		if p.Phase != slskd.PhaseFetching {
			return
		}
		fetching++
		if fetching == 2 {
			fake.SetSearch("Completed", 1, user("bob"))
		}
	}

	responses, err := fake.Client().RunSearch(context.Background(), "low trust", every, progress)
	if err != nil {
		t.Fatalf("RunSearch: %v", err)
	}

	if got := usernames(responses); !slices.Equal(got, []string{"bob"}) {
		t.Errorf("responses from %v, want bob", got)
	}
	if fetching != 2 {
		t.Errorf("read the responses again %d times, want 2", fetching)
	}
}

func TestRunSearch_TakesWhatThereIsWhenSlskdNeverFinishes(t *testing.T) {
	fake := slskdtest.New(t)
	fake.SetSearch("InProgress", 1, user("bob"))
	polls := 0

	responses, err := fake.Client().RunSearch(context.Background(), "low trust", every, func(slskd.SearchProgress) { polls++ })
	if err != nil {
		t.Fatalf("RunSearch: %v", err)
	}

	if got := usernames(responses); !slices.Equal(got, []string{"bob"}) {
		t.Errorf("responses from %v, want bob", got)
	}
	// A minute at the period the core polls with.
	if polls != 119 {
		t.Errorf("gave up after %d polls left it waiting, want 119", polls)
	}
}

func TestRunSearch_StopsWhenItsContextEnds(t *testing.T) {
	fake := slskdtest.New(t)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancelled once the search is seen running: it would poll for 120 periods.
	_, err := fake.Client().RunSearch(ctx, "low trust", every, func(slskd.SearchProgress) { cancel() })

	if !errors.Is(err, context.Canceled) {
		t.Errorf("RunSearch = %v, want context.Canceled", err)
	}
}
