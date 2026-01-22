package importer

import (
	"fmt"
	"strconv"

	"github.com/Sorrow446/go-mp4tag"
)

// writeM4ATags writes MP4/M4A tags using go-mp4tag.
func writeM4ATags(path string, data TagData) error {
	mp4, err := mp4tag.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer mp4.Close()

	// Build custom tags map for freeform iTunes atoms
	custom := make(map[string]string)

	addCustom := func(key, value string) {
		if value != "" {
			custom[key] = value
		}
	}

	// Date tags (ORIGINALDATE/ORIGINALYEAR as freeform atoms)
	addCustom("ORIGINALDATE", data.OriginalDate)
	if data.OriginalDate != "" && len(data.OriginalDate) >= 4 {
		addCustom("ORIGINALYEAR", data.OriginalDate[:4])
	}

	// Release info
	addCustom("LABEL", data.Label)
	addCustom("CATALOGNUMBER", data.CatalogNumber)
	addCustom("BARCODE", data.Barcode)
	addCustom("MEDIA", data.Media)
	addCustom("RELEASESTATUS", data.ReleaseStatus)
	addCustom("RELEASETYPE", data.ReleaseType)
	addCustom("SCRIPT", data.Script)
	addCustom("RELEASECOUNTRY", data.Country)

	// MusicBrainz IDs - use Picard/Mutagen standard names (mixed case with spaces)
	addCustom("MusicBrainz Artist Id", data.MBArtistID)
	addCustom("MusicBrainz Album Id", data.MBReleaseID)
	addCustom("MusicBrainz Release Group Id", data.MBReleaseGroupID)
	addCustom("MusicBrainz Release Track Id", data.MBTrackID)
	addCustom("MusicBrainz Track Id", data.MBRecordingID)

	// Recording info
	addCustom("ISRC", data.ISRC)

	// Track/disc totals as custom (go-mp4tag doesn't have dedicated fields)
	if data.TotalTracks > 0 {
		addCustom("TOTALTRACKS", strconv.Itoa(data.TotalTracks))
	}
	if data.TotalDiscs > 0 {
		addCustom("TOTALDISCS", strconv.Itoa(data.TotalDiscs))
	}

	tags := &mp4tag.MP4Tags{
		Title:       data.Title,
		Artist:      data.Artist,
		Album:       data.Album,
		AlbumArtist: data.AlbumArtist,
		ArtistSort:  data.ArtistSortName,
		TrackNumber: safeInt16(data.TrackNumber),
		TrackTotal:  safeInt16(data.TotalTracks),
		DiscNumber:  safeInt16(data.DiscNumber),
		DiscTotal:   safeInt16(data.TotalDiscs),
		Date:        data.Date,
		CustomGenre: data.Genre,
		Custom:      custom,
	}

	// Add cover art if provided
	if len(data.CoverArt) > 0 {
		tags.Pictures = []*mp4tag.MP4Picture{
			{Data: data.CoverArt},
		}
	}

	if err := mp4.Write(tags, nil); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

// safeInt16 converts int to int16 with bounds checking.
func safeInt16(n int) int16 {
	if n > 32767 {
		return 32767
	}
	if n < -32768 {
		return -32768
	}
	return int16(n)
}
