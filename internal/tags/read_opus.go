package tags

import (
	"path/filepath"

	"go.senan.xyz/taglib"
)

// readOpusWithTaglib reads Opus metadata using TagLib as fallback when dhowden/tag fails.
func readOpusWithTaglib(path string) (*Tag, error) {
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

	t := &Tag{
		Path:        path,
		Title:       title,
		Artist:      artist,
		AlbumArtist: albumArtist,
		Album:       tags.get(taglib.Album),
		Genre:       tags.get(taglib.Genre),
		TrackNumber: tags.getInt(taglib.TrackNumber),
		TotalTracks: tags.getInt("TOTALTRACKS"),
		DiscNumber:  tags.getInt(taglib.DiscNumber),
		TotalDiscs:  tags.getInt("TOTALDISCS"),
	}

	// Read extended tags
	readOpusExtendedTags(path, t)

	return t, nil
}

// readOpusExtendedTags reads extended Vorbis comments from an Opus file using TagLib.
func readOpusExtendedTags(path string, t *Tag) {
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
	t.Label = tags.get(taglib.Label)
	t.CatalogNumber = tags.get(taglib.CatalogNumber)
	t.Barcode = tags.get(taglib.Barcode)
	t.Media = tags.get(taglib.Media)
	t.ReleaseStatus = tags.get(taglib.ReleaseStatus)
	t.ReleaseType = tags.get(taglib.ReleaseType)
	t.Script = tags.get(taglib.Script)
	t.Country = tags.get(taglib.ReleaseCountry)
	t.ISRC = tags.get(taglib.ISRC)

	// MusicBrainz IDs
	t.MBArtistID = tags.get(taglib.MusicBrainzArtistID)
	t.MBReleaseID = tags.get(taglib.MusicBrainzAlbumID)
	t.MBReleaseGroupID = tags.get(taglib.MusicBrainzReleaseGroupID)
	t.MBRecordingID = tags.get(taglib.MusicBrainzTrackID) // Recording ID uses MUSICBRAINZ_TRACKID
	t.MBTrackID = tags.get(taglib.MusicBrainzReleaseTrackID)

	// Track/disc totals (dhowden/tag may not return these)
	if t.TotalTracks == 0 {
		t.TotalTracks = tags.getInt("TOTALTRACKS")
	}
	if t.TotalDiscs == 0 {
		t.TotalDiscs = tags.getInt("TOTALDISCS")
	}
}
