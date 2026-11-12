package core

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/llehouerou/waves/internal/downloads"
	"github.com/llehouerou/waves/internal/slskd"
)

// bobsAlbum is the response of bob holding one file of his "Album" folder.
func bobsAlbum() slskd.SearchResponse {
	return slskd.SearchResponse{Username: "bob", HasFreeSlot: true, Files: []slskd.File{{Filename: remote("01.flac"), Size: 3}}}
}

func TestSearch_RunsToItsEndWithNoClientPollingIt(t *testing.T) {
	c, fake, _ := newDownloadsCore(t, Options{})
	fake.SetSearch("Completed", 1, bobsAlbum())

	if err := c.Downloads().Search("low trust"); err != nil {
		t.Fatalf("search: %v", err)
	}

	if e := nextEvent[SearchProgressed](t, c); e.Query != "low trust" || e.Responses != 1 || e.State != "Completed" {
		t.Errorf("first SearchProgressed = %+v, want low trust, completed with one response", e)
	}
	finished := nextEvent[SearchFinished](t, c)
	if finished.Err != nil || finished.Query != "low trust" {
		t.Fatalf("SearchFinished = %+v, want the end of low trust", finished)
	}
	if len(finished.Responses) != 1 || finished.Responses[0].Username != "bob" {
		t.Errorf("responses = %+v, want bob's", finished.Responses)
	}

	// What the client makes of the responses: one result, bob's folder.
	results, stats := FilterAndScoreResults(finished.Responses, FilterOptions{FilterNoSlot: true})
	if len(results) != 1 || results[0].Directory != `@@bob\Music\Album` || stats.TotalResponses != 1 {
		t.Errorf("results = %+v (stats %+v), want bob's Album folder", results, stats)
	}
}

func TestSearch_ANewSearchReplacesTheOneRunning(t *testing.T) {
	c, fake, _ := newDownloadsCore(t, Options{})
	dl := c.Downloads()

	// slskd leaves the first search in progress.
	if err := dl.Search("first"); err != nil {
		t.Fatalf("search: %v", err)
	}
	nextEvent[SearchProgressed](t, c)

	if err := dl.Search("second"); err != nil {
		t.Fatalf("search: %v", err)
	}
	fake.SetSearch("Completed", 0)

	// slskd has now finished both, but only the second is still followed.
	var finished []string
	deadline := time.After(5 * time.Second)
	for waiting := true; waiting; {
		select {
		case e := <-c.Events():
			if end, ok := e.(SearchFinished); ok {
				finished = append(finished, end.Query)
				deadline = time.After(quiet) // time for the other one to end too
			}
		case <-deadline:
			waiting = false
		}
	}
	if !slices.Equal(finished, []string{"second"}) {
		t.Errorf("searches finished = %v, want second only", finished)
	}
}

func TestSearch_AFailureIsTheEndOfTheSearch(t *testing.T) {
	c, fake, _ := newDownloadsCore(t, Options{})
	fake.Down(true)

	if err := c.Downloads().Search("low trust"); err != nil {
		t.Fatalf("search: %v", err)
	}

	if e := nextEvent[SearchFinished](t, c); e.Err == nil || e.Query != "low trust" || e.Responses != nil {
		t.Errorf("SearchFinished = %+v, want the failure of low trust", e)
	}
}

func TestSearch_WithoutSlskdIsRefused(t *testing.T) {
	c, _ := newTestCore(t, Options{})

	if err := c.Downloads().Search("low trust"); !errors.Is(err, downloads.ErrNoSlskd) {
		t.Errorf("Search = %v, want ErrNoSlskd", err)
	}
}
