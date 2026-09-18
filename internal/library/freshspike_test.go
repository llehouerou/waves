package library

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode"

	_ "modernc.org/sqlite"
)

// Throwaway spike: how many ListenBrainz fresh releases match my library artists?
// go test ./internal/library -run FreshSpike -v
func TestFreshSpike(t *testing.T) {
	if os.Getenv("SPIKE") == "" {
		t.Skip("set SPIKE=1")
	}

	db, err := sql.Open("sqlite", os.Getenv("HOME")+"/.local/share/waves/waves.db?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT DISTINCT album_artist FROM library_tracks`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	mine := map[string]string{}
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		if k := normArtist(a); k != "" {
			mine[k] = a
		}
	}
	t.Logf("library artists: %d", len(mine))

	resp, err := http.Get("https://api.listenbrainz.org/1/explore/fresh-releases?days=90&past=true&future=true&sort=release_date")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var payload struct {
		Payload struct {
			Releases []struct {
				Artist  string   `json:"artist_credit_name"`
				MBIDs   []string `json:"artist_mbids"`
				Name    string   `json:"release_name"`
				Date    string   `json:"release_date"`
				Type    string   `json:"release_group_primary_type"`
				RGMBID  string   `json:"release_group_mbid"`
				Listens int      `json:"listen_count"`
			} `json:"releases"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	all := payload.Payload.Releases
	t.Logf("fresh releases worldwide (90d): %d", len(all))

	// albums déjà possédés + nombre d'albums par artiste (affinité)
	owned := map[string]bool{}
	albumCount := map[string]int{}
	arows, err := db.Query(`SELECT DISTINCT album_artist, album FROM library_tracks`)
	if err != nil {
		t.Fatal(err)
	}
	defer arows.Close()
	for arows.Next() {
		var ar, al string
		if err := arows.Scan(&ar, &al); err != nil {
			t.Fatal(err)
		}
		owned[normArtist(ar)+"|"+normArtist(al)] = true
		albumCount[normArtist(ar)]++
	}

	type hit struct {
		artist, name, date, typ string
		have                    bool
		nAlbums                 int
	}
	var hits []hit
	skipped := map[string]int{}
	for _, r := range all {
		key := normArtist(r.Artist)
		if _, ok := mine[key]; !ok {
			continue
		}
		if r.Artist == "Various Artists" {
			skipped["various"]++
			continue
		}
		if r.Type != "Album" && r.Type != "EP" {
			skipped[r.Type]++
			continue
		}
		hits = append(hits, hit{
			artist:  r.Artist,
			name:    r.Name,
			date:    r.Date,
			typ:     r.Type,
			have:    owned[key+"|"+normArtist(r.Name)],
			nAlbums: albumCount[key],
		})
	}
	t.Logf("skipped: %v  matches: %d", skipped, len(hits))

	// tri: à venir en premier (date croissante), puis sorties récentes (date décroissante)
	today := time.Now().Format("2006-01-02")
	var future, past []hit
	for _, h := range hits {
		if h.date > today {
			future = append(future, h)
		} else {
			past = append(past, h)
		}
	}
	sort.Slice(future, func(i, j int) bool { return future[i].date < future[j].date })
	sort.Slice(past, func(i, j int) bool { return past[i].date > past[j].date })

	line := func(h hit) {
		mark := " "
		if h.have {
			mark = "✓"
		}
		fmt.Printf("  %s  %s  %-26s  %-42s  %-5s  %2d alb.\n",
			h.date, mark, trunc(h.artist, 26), trunc(h.name, 42), h.typ, h.nAlbums)
	}
	fmt.Printf("\n  À VENIR (%d)\n", len(future))
	for i, h := range future {
		if i >= 15 {
			fmt.Printf("  … +%d\n", len(future)-15)
			break
		}
		line(h)
	}
	fmt.Printf("\n  SORTIES RÉCENTES (%d)\n", len(past))
	for i, h := range past {
		if i >= 45 {
			fmt.Printf("  … +%d\n", len(past)-45)
			break
		}
		line(h)
	}
}

// NormalizeTitle s'appuie sur \w (ASCII only) : tout nom non-latin devient ""
// et collisionne. Normalisation unicode-aware pour le matching d'artistes.
func normArtist(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteRune(' ')
			}
			space = false
			b.WriteRune(r)
			continue
		}
		space = true
	}
	return b.String()
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
