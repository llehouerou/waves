package tags

import (
	"fmt"
	"strconv"

	"go.senan.xyz/taglib"
)

// Custom tag keys not in taglib constants
const (
	totalTracks  = "TOTALTRACKS"
	totalDiscs   = "TOTALDISCS"
	originalYear = "ORIGINALYEAR"
)

// writeOpusTags writes Vorbis comments to an Opus file using TagLib.
func writeOpusTags(path string, t *Tag) error {
	tags := make(map[string][]string)

	// Helper to add tag if non-empty
	addTag := func(key, value string) {
		if value != "" {
			tags[key] = []string{value}
		}
	}

	// Helper to add int tag if > 0
	addIntTag := func(key string, value int) {
		if value > 0 {
			tags[key] = []string{strconv.Itoa(value)}
		}
	}

	// Basic tags
	addTag(taglib.Artist, t.Artist)
	addTag(taglib.AlbumArtist, t.AlbumArtist)
	addTag(taglib.Album, t.Album)
	addTag(taglib.Title, t.Title)
	addTag(taglib.Genre, t.Genre)

	// Track/disc numbers
	addIntTag(taglib.TrackNumber, t.TrackNumber)
	addIntTag(totalTracks, t.TotalTracks)
	addIntTag(taglib.DiscNumber, t.DiscNumber)
	addIntTag(totalDiscs, t.TotalDiscs)

	// Date tags
	addTag(taglib.Date, t.Date)
	addTag(taglib.OriginalDate, t.OriginalDate)
	// ORIGINALYEAR is just the year portion of ORIGINALDATE
	if t.OriginalDate != "" && len(t.OriginalDate) >= 4 {
		addTag(originalYear, t.OriginalDate[:4])
	}

	// Artist info
	addTag(taglib.ArtistSort, t.ArtistSortName)

	// Release info
	addTag(taglib.Label, t.Label)
	addTag(taglib.CatalogNumber, t.CatalogNumber)
	addTag(taglib.Barcode, t.Barcode)
	addTag(taglib.Media, t.Media)
	addTag(taglib.ReleaseStatus, t.ReleaseStatus)
	addTag(taglib.ReleaseType, t.ReleaseType)
	addTag(taglib.Script, t.Script)
	addTag(taglib.ReleaseCountry, t.Country)

	// MusicBrainz IDs
	addTag(taglib.MusicBrainzArtistID, t.MBArtistID)
	addTag(taglib.MusicBrainzAlbumID, t.MBReleaseID)
	addTag(taglib.MusicBrainzReleaseGroupID, t.MBReleaseGroupID)
	addTag(taglib.MusicBrainzReleaseTrackID, t.MBTrackID)
	addTag(taglib.MusicBrainzTrackID, t.MBRecordingID) // Recording ID uses MUSICBRAINZ_TRACKID

	// Recording info
	addTag(taglib.ISRC, t.ISRC)

	// Write tags (Clear removes any existing tags not in our map)
	if err := taglib.WriteTags(path, tags, taglib.Clear); err != nil {
		return fmt.Errorf("write tags: %w", err)
	}

	// Write cover art if provided
	if len(t.CoverArt) > 0 {
		if err := taglib.WriteImage(path, t.CoverArt); err != nil {
			return fmt.Errorf("write cover art: %w", err)
		}
	}

	return nil
}
