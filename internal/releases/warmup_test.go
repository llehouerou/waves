package releases

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	lastfmapi "github.com/shkh/lastfm-go/lastfm"

	"github.com/llehouerou/waves/internal/lastfm"
)

// fakeFetcher answers every seed with the same result, recording what it was asked.
type fakeFetcher struct {
	mu    sync.Mutex
	asked []string
	err   error
}

func (f *fakeFetcher) GetSimilarArtists(artist string, _ int) ([]lastfm.SimilarArtist, error) {
	f.mu.Lock()
	f.asked = append(f.asked, artist)
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return []lastfm.SimilarArtist{{Name: "Similar Of " + artist, MatchScore: 0.9}}, nil
}

func (f *fakeFetcher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked)
}

func runWarmup(t *testing.T, db *sql.DB, client SimilarFetcher) WarmupProgress {
	t.Helper()
	ch := make(chan WarmupProgress)
	var last WarmupProgress
	done := make(chan struct{})
	go func() {
		for p := range ch {
			last = p
		}
		close(done)
	}()
	if err := Warmup(WarmupParams{DB: db, Client: client, TTLDays: 7, Progress: ch}); err != nil {
		t.Fatalf("Warmup: %v", err)
	}
	<-done
	return last
}

// manyArtists builds n distinct library artist names.
func manyArtists(n int) []string {
	artists := make([]string, n)
	for i := range artists {
		artists[i] = fmt.Sprintf("Artist %03d", i)
	}
	return artists
}

func seedLibrary(t *testing.T, db *sql.DB, artists ...string) {
	t.Helper()
	for _, a := range artists {
		if _, err := db.Exec(`INSERT INTO library_tracks (album_artist) VALUES (?)`, a); err != nil {
			t.Fatalf("insert library artist: %v", err)
		}
	}
}

func TestWarmup_FetchesStaleSeedsOnly(t *testing.T) {
	db := setupTestDB(t)
	seedLibrary(t, db, "Mogwai", "Portishead", "Portishead")
	// Portishead already has a fresh cache row.
	_, err := db.Exec(
		`INSERT INTO lastfm_similar_artists (artist, similar_artist, match_score, fetched_at) VALUES (?, ?, ?, ?)`,
		"Portishead", "Massive Attack", 0.9, time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("insert fresh cache row: %v", err)
	}

	client := &fakeFetcher{}
	progress := runWarmup(t, db, client)

	if client.count() != 1 || client.asked[0] != "Mogwai" {
		t.Fatalf("asked %v, want only Mogwai", client.asked)
	}
	if progress.Current != 1 || progress.Total != 1 || progress.Failed != 0 {
		t.Errorf("progress = %+v, want 1/1 with no failure", progress)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM lastfm_similar_artists WHERE artist = 'Mogwai'`); n != 1 {
		t.Errorf("Mogwai similar rows = %d, want 1", n)
	}
}

func TestWarmup_LastfmStopCodesCancelTheJob(t *testing.T) {
	for _, code := range []int{codeSuspendedKey, codeRateLimit} {
		db := setupTestDB(t)
		seedLibrary(t, db, manyArtists(200)...)

		client := &fakeFetcher{err: &lastfmapi.LastfmError{Code: code, Message: "stop"}}
		progress := runWarmup(t, db, client)

		if !progress.Canceled {
			t.Errorf("code %d: job not canceled", code)
		}
		if client.count() >= 200 {
			t.Errorf("code %d: kept pushing seeds (%d asked of 200)", code, client.count())
		}
	}
}

func TestWarmup_OtherErrorsOnlySkipTheSeed(t *testing.T) {
	db := setupTestDB(t)
	seedLibrary(t, db, manyArtists(20)...)

	// Code 6: unknown artist. The seed writes no row and is retried next pass.
	client := &fakeFetcher{err: &lastfmapi.LastfmError{Code: 6, Message: "artist not found"}}
	progress := runWarmup(t, db, client)

	if progress.Canceled {
		t.Error("unknown-artist error canceled the job")
	}
	if client.count() != 20 || progress.Failed != 20 {
		t.Errorf("asked %d seeds, %d failed, want 20/20", client.count(), progress.Failed)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM lastfm_similar_artists`); n != 0 {
		t.Errorf("failing seeds wrote %d rows, want 0", n)
	}
}

func TestWarmup_NoClientIsANoop(t *testing.T) {
	db := setupTestDB(t)
	seedLibrary(t, db, "Mogwai")
	if got := runWarmup(t, db, nil); got != (WarmupProgress{}) {
		t.Errorf("progress = %+v, want zero without a Last.fm key", got)
	}
}

func TestStopsJob_IgnoresPlainErrors(t *testing.T) {
	if stopsJob(errors.New("connection refused")) {
		t.Error("a transport error should not cancel the job")
	}
}
