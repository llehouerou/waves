// Throwaway PROTOTYPE (issue #61) — mockup of the releases list.
// Not production code: no tests, no real error handling.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	_ "modernc.org/sqlite"
)

type release struct {
	Artist    string
	Title     string
	Date      string // YYYY-MM-DD
	Type      string
	RGMBID    string
	Owned     bool     // album already in the library
	Discovery bool     // artist outside the library
	From      []string // seeds recommending it (discoveries only)
}

const cachePath = "/tmp/waves-releases-PROTOTYPE-wipe-me.json"

// load returns the releases, from the on-disk cache when present.
func load() ([]release, error) {
	if b, err := os.ReadFile(cachePath); err == nil {
		var rs []release
		if json.Unmarshal(b, &rs) == nil && len(rs) > 0 {
			return rs, nil
		}
	}
	rs, err := build()
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(rs)
	_ = os.WriteFile(cachePath, b, 0o600)
	return rs, nil
}

func build() ([]release, error) {
	db, err := sql.Open("sqlite", os.Getenv("HOME")+"/.local/share/waves/waves.db?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	mine := map[string]string{} // normalized name -> display name
	owned := map[string]bool{}  // normalized artist|album
	rows, err := db.Query(`SELECT DISTINCT album_artist, album FROM library_tracks`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ar, al string
		if err := rows.Scan(&ar, &al); err != nil {
			return nil, err
		}
		k := normArtist(ar)
		if k == "" {
			continue
		}
		mine[k] = ar
		owned[k+"|"+normArtist(al)] = true
	}
	rows.Close() //nolint:sqlclosecheck // prototype: sequential queries, closed explicitly

	// Discoveries: existing Last.fm cache (decision #60), artists recommended by >= 2 seeds.
	sims := map[string][]string{}
	names := map[string]string{}
	srows, err := db.Query(`SELECT artist, similar_artist FROM lastfm_similar_artists`)
	if err != nil {
		return nil, err
	}
	for srows.Next() {
		var seed, sim string
		if err := srows.Scan(&seed, &sim); err != nil {
			return nil, err
		}
		k := normArtist(sim)
		if k == "" || mine[k] != "" {
			continue
		}
		names[k] = sim
		sims[k] = append(sims[k], seed)
	}
	srows.Close() //nolint:sqlclosecheck // prototype: sequential queries, closed explicitly

	fresh, err := fetchFresh()
	if err != nil {
		return nil, err
	}

	var out []release
	for _, r := range fresh {
		if r.Artist == "Various Artists" || (r.Type != "Album" && r.Type != "EP") { //nolint:goconst // prototype
			continue
		}
		k := normArtist(r.Artist)
		switch {
		case mine[k] != "":
			out = append(out, release{
				Artist: r.Artist, Title: r.Name, Date: r.Date, Type: r.Type,
				RGMBID: r.RGMBID, Owned: owned[k+"|"+normArtist(r.Name)],
			})
		case len(sims[k]) >= 2:
			from := sims[k]
			sort.Strings(from)
			out = append(out, release{
				Artist: names[k], Title: r.Name, Date: r.Date, Type: r.Type,
				RGMBID: r.RGMBID, Discovery: true, From: from,
			})
		}
	}
	sortReleases(out)
	return out, nil
}

// sortReleases: upcoming first (soonest first), then past (most recent first).
func sortReleases(rs []release) {
	today := time.Now().Format("2006-01-02")
	sort.SliceStable(rs, func(i, j int) bool {
		fi, fj := rs[i].Date > today, rs[j].Date > today
		if fi != fj {
			return fi
		}
		if fi {
			return rs[i].Date < rs[j].Date
		}
		return rs[i].Date > rs[j].Date
	})
}

type lbRelease struct {
	Artist string `json:"artist_credit_name"`
	Name   string `json:"release_name"`
	Date   string `json:"release_date"`
	Type   string `json:"release_group_primary_type"`
	RGMBID string `json:"release_group_mbid"`
}

func fetchFresh() ([]lbRelease, error) {
	req, _ := http.NewRequest(http.MethodGet,
		"https://api.listenbrainz.org/1/explore/fresh-releases?days=90&past=true&future=true&sort=release_date", http.NoBody)
	req.Header.Set("User-Agent", "waves-prototype/0.1 (https://github.com/llehouerou/waves)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listenbrainz: %s", resp.Status)
	}
	var payload struct {
		Payload struct {
			Releases []lbRelease `json:"releases"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Payload.Releases, nil
}

// normArtist: lowercase, unicode letters/digits only, collapsed spaces.
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

// stressRows: synthetic rows to judge CJK and very long titles (key x).
func stressRows() []release {
	today := time.Now()
	d := func(n int) string { return today.AddDate(0, 0, n).Format("2006-01-02") }
	return []release{
		{Artist: "宇多田ヒカル", Title: "科学者たちの夜明け (Deluxe Edition)", Date: d(3), Type: "Album"},
		{Artist: "김광석", Title: "서른 즈음에", Date: d(-2), Type: "EP", Discovery: true, From: []string{"Sigur Rós", "Mogwai"}},
		{Artist: "Godspeed You! Black Emperor", Title: "Lift Your Skinny Fists Like Antennas to Heaven (25th Anniversary Remaster)", Date: d(-1), Type: "Album", Owned: true},
		{Artist: "Sigur Rós", Title: "Ágætis byrjun", Date: d(0), Type: "Album", Discovery: true, From: []string{"Mogwai", "Explosions in the Sky", "Slint", "Godspeed You! Black Emperor"}},
	}
}
