package tags

import (
	"fmt"
	"strconv"

	"github.com/Sorrow446/go-mp4tag"
)

// writeM4ATags writes MP4/M4A tags using go-mp4tag.
func writeM4ATags(path string, t *Tag) error {
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
	addCustom("ORIGINALDATE", t.OriginalDate)
	if t.OriginalDate != "" && len(t.OriginalDate) >= 4 {
		addCustom("ORIGINALYEAR", t.OriginalDate[:4])
	}

	// Release info
	addCustom("LABEL", t.Label)
	addCustom("CATALOGNUMBER", t.CatalogNumber)
	addCustom("BARCODE", t.Barcode)
	addCustom("MEDIA", t.Media)
	addCustom("RELEASESTATUS", t.ReleaseStatus)
	addCustom("RELEASETYPE", t.ReleaseType)
	addCustom("SCRIPT", t.Script)
	addCustom("RELEASECOUNTRY", t.Country)

	// MusicBrainz IDs - use Picard/Mutagen standard names (mixed case with spaces)
	addCustom("MusicBrainz Artist Id", t.MBArtistID)
	addCustom("MusicBrainz Album Id", t.MBReleaseID)
	addCustom("MusicBrainz Release Group Id", t.MBReleaseGroupID)
	addCustom("MusicBrainz Release Track Id", t.MBTrackID)
	addCustom("MusicBrainz Track Id", t.MBRecordingID)

	// Recording info
	addCustom("ISRC", t.ISRC)

	// Track/disc totals as custom atoms (redundant with standard fields below,
	// but some players only read freeform atoms)
	if t.TotalTracks > 0 {
		addCustom("TOTALTRACKS", strconv.Itoa(t.TotalTracks))
	}
	if t.TotalDiscs > 0 {
		addCustom("TOTALDISCS", strconv.Itoa(t.TotalDiscs))
	}

	tags := &mp4tag.MP4Tags{
		Title:       t.Title,
		Artist:      t.Artist,
		Album:       t.Album,
		AlbumArtist: t.AlbumArtist,
		ArtistSort:  t.ArtistSortName,
		TrackNumber: safeInt16(t.TrackNumber),
		TrackTotal:  safeInt16(t.TotalTracks),
		DiscNumber:  safeInt16(t.DiscNumber),
		DiscTotal:   safeInt16(t.TotalDiscs),
		Date:        t.Date,
		CustomGenre: t.Genre,
		Custom:      custom,
	}

	// Add cover art if provided
	if len(t.CoverArt) > 0 {
		tags.Pictures = []*mp4tag.MP4Picture{
			{Data: t.CoverArt},
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
