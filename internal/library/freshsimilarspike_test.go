package library

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/llehouerou/waves/internal/config"
	"github.com/llehouerou/waves/internal/lastfm"
)

type lbRelease struct {
	Artist  string   `json:"artist_credit_name"`
	MBIDs   []string `json:"artist_mbids"`
	Name    string   `json:"release_name"`
	Date    string   `json:"release_date"`
	Type    string   `json:"release_group_primary_type"`
	RGMBID  string   `json:"release_group_mbid"`
	Listens int      `json:"listen_count"`
}

func fetchFresh(t *testing.T, days int) []lbRelease {
	t.Helper()
	url := fmt.Sprintf("https://api.listenbrainz.org/1/explore/fresh-releases?days=%d&past=true&future=true&sort=release_date", days)
	resp, err := http.Get(url) //nolint:gosec,noctx // spike
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var payload struct {
		Payload struct {
			Releases []lbRelease `json:"releases"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload.Payload.Releases
}

// Spike 2: étendre aux artistes similaires (Last.fm) absents de la librairie.
// SPIKE=1 go test ./internal/library -run FreshSimilarSpike -v
func TestFreshSimilarSpike(t *testing.T) {
	if os.Getenv("SPIKE") == "" {
		t.Skip("set SPIKE=1")
	}

	db, err := sql.Open("sqlite", os.Getenv("HOME")+"/.local/share/waves/waves.db?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// artistes de la librairie, triés par nombre de pistes (proxy d'affinité)
	rows, err := db.Query(`
		SELECT album_artist, COUNT(*) FROM library_tracks
		GROUP BY album_artist ORDER BY COUNT(*) DESC`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	owned := map[string]bool{}
	var seeds []string
	for rows.Next() {
		var a string
		var n int
		if err := rows.Scan(&a, &n); err != nil {
			t.Fatal(err)
		}
		k := normArtist(a)
		if k == "" || a == "Various Artists" {
			continue
		}
		owned[k] = true
		if len(seeds) < seedCount {
			seeds = append(seeds, a)
		}
	}
	t.Logf("librairie: %d artistes, %d seeds", len(owned), len(seeds))

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	lf := lastfm.New(cfg.Lastfm.APIKey, cfg.Lastfm.APISecret)

	// similaires: score max + seeds qui les recommandent
	type sim struct {
		name  string
		score float64
		from  []string
	}
	var mu sync.Mutex
	sims := map[string]*sim{}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	start := time.Now()
	for _, seed := range seeds {
		wg.Add(1)
		go func(seed string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			list, err := lf.GetSimilarArtists(seed, 30)
			if err != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, s := range list {
				k := normArtist(s.Name)
				if k == "" || owned[k] {
					continue
				}
				cur := sims[k]
				if cur == nil {
					cur = &sim{name: s.Name}
					sims[k] = cur
				}
				cur.score = max(cur.score, s.MatchScore)
				cur.from = append(cur.from, seed)
			}
		}(seed)
	}
	wg.Wait()
	t.Logf("similaires hors librairie: %d (%.1fs de Last.fm)", len(sims), time.Since(start).Seconds())

	all := fetchFresh(t, 90)
	t.Logf("fresh releases: %d", len(all))

	type hit struct {
		artist, name, date, typ string
		score                   float64
		from                    []string
	}
	var hits []hit
	for _, r := range all {
		s := sims[normArtist(r.Artist)]
		if s == nil || (r.Type != "Album" && r.Type != "EP") {
			continue
		}
		hits = append(hits, hit{r.Artist, r.Name, r.Date, r.Type, s.score, s.from})
	}
	sort.Slice(hits, func(i, j int) bool {
		if len(hits[i].from) != len(hits[j].from) {
			return len(hits[i].from) > len(hits[j].from)
		}
		return hits[i].score > hits[j].score
	})

	fmt.Printf("\n  DÉCOUVERTES (%d) — artistes similaires absents de la librairie\n", len(hits))
	for i, h := range hits {
		if i >= 40 {
			fmt.Printf("  … +%d\n", len(hits)-40)
			break
		}
		fmt.Printf("  %s  %-24s  %-38s  %-5s  %.2f  ← %s\n",
			h.date, trunc(h.artist, 24), trunc(h.name, 38), h.typ, h.score, trunc(joinSeeds(h.from), 40))
	}
}

const seedCount = 80

func joinSeeds(s []string) string {
	if len(s) > 3 {
		return fmt.Sprintf("%s, %s, +%d", s[0], s[1], len(s)-2)
	}
	return strings.Join(s, ", ")
}
