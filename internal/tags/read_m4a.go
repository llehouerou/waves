package tags

import (
	"path/filepath"

	"go.senan.xyz/taglib"
)

// readM4AWithTaglib reads M4A metadata using TagLib as fallback when dhowden/tag fails.
func readM4AWithTaglib(path string) (*Tag, error) {
	rawTags, err := taglib.ReadTags(path)
	if err != nil {
		return nil, err
	}
	tags := taglibTags(rawTags)

	title := tags.get(taglib.Title)
	if title == "" {
		title = filepath.Base(path)
	}

	artist := tags.get(taglib.Artist)
	albumArtist := tags.get(taglib.AlbumArtist)
	if albumArtist == "" {
		albumArtist = artist
	}

	trackNum, trackTotal := tags.parseNumberPair(taglib.TrackNumber)
	discNum, discTotal := tags.parseNumberPair(taglib.DiscNumber)

	// Also check custom TOTALTRACKS/TOTALDISCS atoms if not in the number format
	if trackTotal == 0 {
		trackTotal = tags.getInt("TOTALTRACKS")
	}
	if discTotal == 0 {
		discTotal = tags.getInt("TOTALDISCS")
	}

	t := &Tag{
		Path:        path,
		Title:       title,
		Artist:      artist,
		AlbumArtist: albumArtist,
		Album:       tags.get(taglib.Album),
		Genre:       tags.get(taglib.Genre),
		TrackNumber: trackNum,
		TotalTracks: trackTotal,
		DiscNumber:  discNum,
		TotalDiscs:  discTotal,
	}

	// Read extended tags
	readM4AExtendedTags(path, t)

	t.Sanitize()
	return t, nil
}

// readM4AExtendedTags reads extended tags from an M4A/MP4 file using TagLib.
func readM4AExtendedTags(path string, t *Tag) {
	rawTags, err := taglib.ReadTags(path)
	if err != nil {
		return
	}
	tags := taglibTags(rawTags)

	// Read extended tags
	t.Date = tags.get(taglib.Date)
	t.OriginalDate = tags.get(taglib.OriginalDate)
	if t.OriginalDate == "" {
		t.OriginalDate = tags.get("ORIGINALYEAR")
	}

	t.ArtistSortName = tags.get(taglib.ArtistSort)
	t.Label = tags.get(taglib.Label, "LABEL")
	t.CatalogNumber = tags.get(taglib.CatalogNumber, "CATALOGNUMBER")
	t.Barcode = tags.get(taglib.Barcode, "BARCODE")
	t.Media = tags.get(taglib.Media, "MEDIA")
	t.ReleaseStatus = tags.get(taglib.ReleaseStatus, "RELEASESTATUS")
	t.ReleaseType = tags.get(taglib.ReleaseType, "RELEASETYPE")
	t.Script = tags.get(taglib.Script, "SCRIPT")
	t.Country = tags.get(taglib.ReleaseCountry, "RELEASECOUNTRY")
	t.ISRC = tags.get(taglib.ISRC, "ISRC")

	// MusicBrainz IDs - try all known formats for compatibility:
	// 1. TagLib underscore format (MUSICBRAINZ_ARTISTID)
	// 2. Uppercase with spaces (MUSICBRAINZ ARTIST ID)
	// 3. Picard/Mutagen standard - mixed case with spaces (MusicBrainz Artist Id)
	t.MBArtistID = tags.get(
		taglib.MusicBrainzArtistID,
		"MUSICBRAINZ ARTIST ID",
		"MusicBrainz Artist Id",
	)
	t.MBReleaseID = tags.get(
		taglib.MusicBrainzAlbumID,
		"MUSICBRAINZ ALBUM ID",
		"MusicBrainz Album Id",
	)
	t.MBReleaseGroupID = tags.get(
		taglib.MusicBrainzReleaseGroupID,
		"MUSICBRAINZ RELEASE GROUP ID",
		"MusicBrainz Release Group Id",
	)
	t.MBRecordingID = tags.get(
		taglib.MusicBrainzTrackID,
		"MUSICBRAINZ TRACK ID",
		"MusicBrainz Track Id",
	)
	t.MBTrackID = tags.get(
		taglib.MusicBrainzReleaseTrackID,
		"MUSICBRAINZ RELEASE TRACK ID",
		"MusicBrainz Release Track Id",
	)
}
