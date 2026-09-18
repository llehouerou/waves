package releases

import (
	"database/sql"
	"fmt"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/listenbrainz"
)

// variousArtists is excluded from the list: compilation credits match nothing useful.
const variousArtists = "various artists"

// Release is a cached release matched against the library.
type Release struct {
	ReleaseGroupMBID string
	ArtistCreditName string
	ReleaseName      string
	ReleaseDate      string
	PrimaryType      string
	SecondaryType    string
	ListenCount      int
	InLibrary        bool // false means a discovery: a similar artist absent from the library
}

// Matched returns the cached releases whose artist is in the library or among the
// similar artists: Album and EP only, Various Artists excluded, inside the 90-day
// window (a stale cache can hold older rows), sorted by date then artist. The
// artist maps are rebuilt on every call, so a newly imported artist shows up
// without refetching.
//
// ponytail: full scan of the ~30 700 cached rows per call, tens of ms.
// Add a normalised column + index on library_tracks if it ever shows.
func (c *Cache) Matched() ([]Release, error) {
	libraryArtists, err := normalizedSet(c.db, `SELECT DISTINCT album_artist FROM library_tracks`)
	if err != nil {
		return nil, err
	}
	// Raw union of the cached seeds; the "recommended by >= 2 seeds" rule lands with #69.
	similarArtists, err := normalizedSet(c.db, `SELECT DISTINCT similar_artist FROM lastfm_similar_artists`)
	if err != nil {
		return nil, err
	}

	rows, err := c.db.Query(`
		SELECT release_group_mbid, artist_credit_name, norm_artist, release_name,
		       release_date, primary_type, secondary_type, listen_count
		FROM fresh_releases
		WHERE primary_type IN ('Album', 'EP')
		  AND release_date BETWEEN date('now', ?) AND date('now', ?)
		ORDER BY release_date, artist_credit_name
	`,
		fmt.Sprintf("-%d days", listenbrainz.WindowDays),
		fmt.Sprintf("+%d days", listenbrainz.WindowDays),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matched []Release
	for rows.Next() {
		var r Release
		var normArtist string
		var primaryType, secondaryType sql.NullString
		if err := rows.Scan(&r.ReleaseGroupMBID, &r.ArtistCreditName, &normArtist, &r.ReleaseName,
			&r.ReleaseDate, &primaryType, &secondaryType, &r.ListenCount); err != nil {
			return nil, err
		}
		if normArtist == variousArtists {
			continue
		}

		inLibrary := libraryArtists[normArtist]
		if !inLibrary && !similarArtists[normArtist] {
			continue
		}

		r.PrimaryType = primaryType.String
		r.SecondaryType = secondaryType.String
		r.InLibrary = inLibrary
		matched = append(matched, r)
	}

	return matched, rows.Err()
}

// normalizedSet reads a single-column artist-name query into a normalised lookup set.
func normalizedSet(db *sql.DB, query string) (map[string]bool, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	set := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if norm := library.NormalizeTitle(name); norm != "" {
			set[norm] = true
		}
	}
	return set, rows.Err()
}
