package releases

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	lastfmapi "github.com/shkh/lastfm-go/lastfm"

	"github.com/llehouerou/waves/internal/lastfm"
	"github.com/llehouerou/waves/internal/radio"
)

const (
	// warmupWorkers is how many seeds are fetched in parallel.
	warmupWorkers = 8
	// similarFetchLimit is how many similar artists a seed is fetched with. The
	// read path keeps 30 of them; radio, sharing the cache, uses the rest.
	similarFetchLimit = 50
)

// Last.fm application errors arrive as HTTP 200 with a code in the body.
const (
	codeSuspendedKey = 26
	codeRateLimit    = 29
)

// SimilarFetcher is the Last.fm surface the warm-up needs.
type SimilarFetcher interface {
	GetSimilarArtists(artist string, limit int) ([]lastfm.SimilarArtist, error)
}

// WarmupProgress reports the state of the similar-artists warm-up job.
type WarmupProgress struct {
	Current  int
	Total    int
	Failed   int
	Canceled bool // Last.fm told us to stop: suspended key or rate limit
}

// WarmupParams configures the similar-artists warm-up job.
type WarmupParams struct {
	DB      *sql.DB
	Client  SimilarFetcher // nil when no Last.fm API key is configured
	TTLDays int
	// Progress must be drained until Warmup closes it: the last send blocks.
	Progress chan<- WarmupProgress
}

// seedResult is one seed's answer on its way to the single writer.
type seedResult struct {
	seed    string
	similar []lastfm.SimilarArtist
	err     error
}

// Warmup fills the shared lastfm_similar_artists cache for library artists whose
// row is missing or stale, warming radio mode at the same time. Each seed is
// written independently: a failing seed writes nothing and is retried on the next
// pass. Closes Progress when done.
func Warmup(p WarmupParams) error {
	defer close(p.Progress)
	if p.Client == nil {
		return nil
	}

	seeds, err := staleSeeds(p.DB, p.TTLDays)
	if err != nil || len(seeds) == 0 {
		return err
	}

	cache := radio.NewCache(p.DB, p.TTLDays)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan string)
	go func() {
		defer close(jobs)
		for _, seed := range seeds {
			select {
			case jobs <- seed:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Workers only fetch; this goroutine is the single writer and the single
	// counter, so the cache never sees eight concurrent transactions.
	results := make(chan seedResult)
	var wg sync.WaitGroup
	for range warmupWorkers {
		wg.Go(func() {
			for seed := range jobs {
				similar, err := p.Client.GetSimilarArtists(seed, similarFetchLimit)
				select {
				case results <- seedResult{seed: seed, similar: similar, err: err}:
				case <-ctx.Done():
					return
				}
			}
		})
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	progress := WarmupProgress{Total: len(seeds)}
	for r := range results {
		progress.Current++
		switch {
		case r.err != nil:
			progress.Failed++
			if stopsJob(r.err) {
				progress.Canceled = true
				cancel() // stop pushing seeds at an API saying no
			}
		default:
			if err := cache.SetSimilarArtists(r.seed, r.similar); err != nil {
				progress.Failed++ // unwritten seed is retried on the next pass
			}
		}

		select { // never stall the job on the UI
		case p.Progress <- progress:
		default:
		}
	}

	p.Progress <- progress
	return nil
}

// stopsJob reports whether Last.fm told us to stop pushing requests.
func stopsJob(err error) bool {
	var apiErr *lastfmapi.LastfmError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Code == codeSuspendedKey || apiErr.Code == codeRateLimit
}

// staleSeeds returns the library artists with no fresh similar-artists cache row.
func staleSeeds(db *sql.DB, ttlDays int) ([]string, error) {
	cutoff := time.Now().AddDate(0, 0, -ttlDays).Unix()
	rows, err := db.Query(`
		SELECT DISTINCT lt.album_artist
		FROM library_tracks lt
		WHERE lt.album_artist != ''
		  AND NOT EXISTS (
			SELECT 1 FROM lastfm_similar_artists s
			WHERE s.artist = lt.album_artist AND s.fetched_at >= ?
		  )
		ORDER BY lt.album_artist
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seeds []string
	for rows.Next() {
		var seed string
		if err := rows.Scan(&seed); err != nil {
			return nil, err
		}
		seeds = append(seeds, seed)
	}
	return seeds, rows.Err()
}
