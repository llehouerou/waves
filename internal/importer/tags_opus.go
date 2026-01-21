package importer

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
func writeOpusTags(path string, data TagData) error {
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
	addTag(taglib.Artist, data.Artist)
	addTag(taglib.AlbumArtist, data.AlbumArtist)
	addTag(taglib.Album, data.Album)
	addTag(taglib.Title, data.Title)
	addTag(taglib.Genre, data.Genre)

	// Track/disc numbers
	addIntTag(taglib.TrackNumber, data.TrackNumber)
	addIntTag(totalTracks, data.TotalTracks)
	addIntTag(taglib.DiscNumber, data.DiscNumber)
	addIntTag(totalDiscs, data.TotalDiscs)

	// Date tags
	addTag(taglib.Date, data.Date)
	addTag(taglib.OriginalDate, data.OriginalDate)
	// ORIGINALYEAR is just the year portion of ORIGINALDATE
	if data.OriginalDate != "" && len(data.OriginalDate) >= 4 {
		addTag(originalYear, data.OriginalDate[:4])
	}

	// Artist info
	addTag(taglib.ArtistSort, data.ArtistSortName)

	// Release info
	addTag(taglib.Label, data.Label)
	addTag(taglib.CatalogNumber, data.CatalogNumber)
	addTag(taglib.Barcode, data.Barcode)
	addTag(taglib.Media, data.Media)
	addTag(taglib.ReleaseStatus, data.ReleaseStatus)
	addTag(taglib.ReleaseType, data.ReleaseType)
	addTag(taglib.Script, data.Script)
	addTag(taglib.ReleaseCountry, data.Country)

	// MusicBrainz IDs
	addTag(taglib.MusicBrainzArtistID, data.MBArtistID)
	addTag(taglib.MusicBrainzAlbumID, data.MBReleaseID)
	addTag(taglib.MusicBrainzReleaseGroupID, data.MBReleaseGroupID)
	addTag(taglib.MusicBrainzReleaseTrackID, data.MBTrackID)
	addTag(taglib.MusicBrainzTrackID, data.MBRecordingID) // Recording ID uses MUSICBRAINZ_TRACKID

	// Recording info
	addTag(taglib.ISRC, data.ISRC)

	// Write tags (Clear removes any existing tags not in our map)
	if err := taglib.WriteTags(path, tags, taglib.Clear); err != nil {
		return fmt.Errorf("write tags: %w", err)
	}

	// Write cover art if provided
	if len(data.CoverArt) > 0 {
		if err := taglib.WriteImage(path, data.CoverArt); err != nil {
			return fmt.Errorf("write cover art: %w", err)
		}
	}

	return nil
}
