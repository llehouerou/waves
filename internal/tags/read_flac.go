package tags

import (
	"path/filepath"
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
	readFLACExtendedTags(path, t)

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
	var comments map[string]string
	for _, meta := range f.Meta {
		if meta.Type == goflac.VorbisComment {
			comments = parseVorbisComments(meta.Data)
			break
		}
	}

	if comments == nil {
		return
	}

	// Read extended tags
	t.Date = comments["DATE"]
	if t.Date == "" {
		// Fallback to YEAR if DATE not present
		t.Date = comments["YEAR"]
	}
	t.OriginalDate = comments["ORIGINALDATE"]
	if t.OriginalDate == "" {
		t.OriginalDate = comments["ORIGINALYEAR"]
	}

	t.ArtistSortName = comments["ARTISTSORT"]
	t.Label = comments["LABEL"]
	t.CatalogNumber = comments["CATALOGNUMBER"]
	t.Barcode = comments["BARCODE"]
	t.Media = comments["MEDIA"]
	t.ReleaseStatus = comments["RELEASESTATUS"]
	t.ReleaseType = comments["RELEASETYPE"]
	t.Script = comments["SCRIPT"]
	t.Country = comments["RELEASECOUNTRY"]
	t.ISRC = comments["ISRC"]

	// MusicBrainz IDs
	t.MBArtistID = comments["MUSICBRAINZ_ARTISTID"]
	t.MBReleaseID = comments["MUSICBRAINZ_ALBUMID"]
	t.MBReleaseGroupID = comments["MUSICBRAINZ_RELEASEGROUPID"]
	t.MBRecordingID = comments["MUSICBRAINZ_TRACKID"]
	t.MBTrackID = comments["MUSICBRAINZ_RELEASETRACKID"]

	// Track/disc totals (dhowden/tag may not return these)
	if t.TotalTracks == 0 {
		if n, err := strconv.Atoi(comments["TOTALTRACKS"]); err == nil {
			t.TotalTracks = n
		}
	}
	if t.TotalDiscs == 0 {
		if n, err := strconv.Atoi(comments["TOTALDISCS"]); err == nil {
			t.TotalDiscs = n
		}
	}
}

// parseVorbisComments parses raw Vorbis comment data into a map.
func parseVorbisComments(data []byte) map[string]string {
	comments := make(map[string]string)

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
			comments[key] = value
		}
	}

	return comments
}
