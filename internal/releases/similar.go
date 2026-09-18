package releases

import (
	"database/sql"
	"sort"

	"github.com/llehouerou/waves/internal/library"
)

const (
	// maxSimilarPerSeed caps how many similar artists one seed contributes.
	maxSimilarPerSeed = 30
	// minRecommenders is how many library seeds must recommend an artist to keep it.
	// No match threshold: Last.fm match scores are not comparable across artists.
	minRecommenders = 2
)

// Discovery is an artist similar to library artists but absent from the library.
type Discovery struct {
	Name  string   // Last.fm name, for display
	Norm  string   // normalised name, for matching
	Seeds []string // library artists that recommended it
}

// discoveries derives the discovery set from the shared lastfm_similar_artists
// cache as it stands: at most maxSimilarPerSeed per library seed, artists
// recommended by at least minRecommenders seeds and absent from the library,
// sorted by recommender count descending.
func discoveries(db *sql.DB, libraryArtists map[string]bool) ([]Discovery, error) {
	rows, err := db.Query(`
		SELECT artist, similar_artist
		FROM lastfm_similar_artists
		ORDER BY artist, match_score DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byNorm := make(map[string]*Discovery)
	perSeed := make(map[string]int)
	for rows.Next() {
		var seed, similar string
		if err := rows.Scan(&seed, &similar); err != nil {
			return nil, err
		}
		normSeed := library.NormalizeTitle(seed)
		if !libraryArtists[normSeed] || perSeed[normSeed] >= maxSimilarPerSeed {
			continue
		}
		perSeed[normSeed]++

		norm := library.NormalizeTitle(similar)
		if norm == "" || libraryArtists[norm] {
			continue // already in the library: not a discovery
		}
		d := byNorm[norm]
		if d == nil {
			d = &Discovery{Name: similar, Norm: norm}
			byNorm[norm] = d
		}
		d.Seeds = append(d.Seeds, seed)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]Discovery, 0, len(byNorm))
	for _, d := range byNorm {
		if len(d.Seeds) >= minRecommenders {
			result = append(result, *d)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if len(result[i].Seeds) != len(result[j].Seeds) {
			return len(result[i].Seeds) > len(result[j].Seeds)
		}
		return result[i].Norm < result[j].Norm
	})
	return result, nil
}
