package tags

import (
	"strconv"
	"strings"

	goflac "github.com/go-flac/go-flac"
	"go.senan.xyz/taglib"
)

// readFLACWithTaglib reads FLAC metadata using TagLib as fallback when dhowden/tag fails.
func readFLACWithTaglib(path string) (*Tag, error) {
	rawTags, err := taglib.ReadTags(path)
	if err != nil {
		return nil, err
	}
	tags := tagValues(rawTags)

	t := &Tag{
		Path:        path,
		Title:       tags.get(taglib.Title),
		Artist:      tags.get(taglib.Artist),
		AlbumArtist: tags.get(taglib.AlbumArtist),
		Album:       tags.get(taglib.Album),
		Genre:       tags.get(taglib.Genre),
		TrackNumber: tags.getInt(taglib.TrackNumber),
		TotalTracks: tags.getInt("TOTALTRACKS"),
		DiscNumber:  tags.getInt(taglib.DiscNumber),
		TotalDiscs:  tags.getInt("TOTALDISCS"),
	}

	// Read extended tags
	readFLACExtendedTags(path, t)

	t.applyFallbacks()
	t.Sanitize()
	return t, nil
}

// readFLACExtendedTags reads extended Vorbis comments from a FLAC file.
func readFLACExtendedTags(path string, t *Tag) {
	f, err := goflac.ParseFile(path)
	if err != nil {
		return
	}

	// Find Vorbis comment block
	var comments tagValues
	for _, meta := range f.Meta {
		if meta.Type == goflac.VorbisComment {
			comments = parseVorbisComments(meta.Data)
			break
		}
	}

	if comments == nil {
		return
	}

	t.joinBasicIfSeveral(comments.basicValues)

	// Read extended tags
	t.Date = comments.get("DATE")
	if t.Date == "" {
		// Fallback to YEAR if DATE not present
		t.Date = comments.get("YEAR")
	}
	t.OriginalDate = comments.get("ORIGINALDATE")
	if t.OriginalDate == "" {
		t.OriginalDate = comments.get("ORIGINALYEAR")
	}

	t.ArtistSortName = comments.get("ARTISTSORT")
	t.Label = comments.get("LABEL")
	t.CatalogNumber = comments.get("CATALOGNUMBER")
	t.Barcode = comments.get("BARCODE")
	t.Media = comments.get("MEDIA")
	t.ReleaseStatus = comments.get("RELEASESTATUS")
	t.ReleaseType = comments.get("RELEASETYPE")
	t.Script = comments.get("SCRIPT")
	t.Country = comments.get("RELEASECOUNTRY")
	t.ISRC = comments.get("ISRC")

	// MusicBrainz IDs
	t.MBArtistID = comments.get("MUSICBRAINZ_ARTISTID")
	t.MBReleaseID = comments.get("MUSICBRAINZ_ALBUMID")
	t.MBReleaseGroupID = comments.get("MUSICBRAINZ_RELEASEGROUPID")
	t.MBRecordingID = comments.get("MUSICBRAINZ_TRACKID")
	t.MBTrackID = comments.get("MUSICBRAINZ_RELEASETRACKID")

	// Track/disc totals (dhowden/tag may not return these)
	if t.TotalTracks == 0 {
		if n, err := strconv.Atoi(comments.first("TOTALTRACKS")); err == nil {
			t.TotalTracks = n
		}
	}
	if t.TotalDiscs == 0 {
		if n, err := strconv.Atoi(comments.first("TOTALDISCS")); err == nil {
			t.TotalDiscs = n
		}
	}
}

// parseVorbisComments parses raw Vorbis comment data into the values of each
// name, in file order. Names are compared without case.
func parseVorbisComments(data []byte) tagValues {
	comments := make(tagValues)

	if len(data) < 4 {
		return comments
	}

	// Skip vendor string
	vendorLen := int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24
	pos := 4 + vendorLen
	if pos+4 > len(data) {
		return comments
	}

	// Read comment count
	commentCount := int(data[pos]) | int(data[pos+1])<<8 | int(data[pos+2])<<16 | int(data[pos+3])<<24
	pos += 4

	// Read each comment
	for i := 0; i < commentCount && pos+4 <= len(data); i++ {
		commentLen := int(data[pos]) | int(data[pos+1])<<8 | int(data[pos+2])<<16 | int(data[pos+3])<<24
		pos += 4

		if pos+commentLen > len(data) {
			break
		}

		comment := string(data[pos : pos+commentLen])
		pos += commentLen

		// Split on first '='
		if idx := strings.Index(comment, "="); idx > 0 {
			key := strings.ToUpper(comment[:idx])
			value := comment[idx+1:]
			comments[key] = append(comments[key], value)
		}
	}

	return comments
}
