package tags

import (
	"strconv"
	"strings"

	"github.com/bogem/id3v2/v2"
	"go.senan.xyz/taglib"
)

// id3BasicFrames maps the property name of a basic tag to its ID3v2 frame.
var id3BasicFrames = map[string]string{
	taglib.Title:       "TIT2",
	taglib.Artist:      "TPE1",
	taglib.AlbumArtist: "TPE2",
	taglib.Album:       "TALB",
	taglib.Genre:       "TCON",
}

// readMP3ExtendedTags reads extended ID3v2 tags from an MP3 file.
func readMP3ExtendedTags(path string, t *Tag) {
	id3tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return
	}
	defer id3tag.Close()

	t.joinBasicIfSeveral(func(name string) []string {
		return getID3TextValues(id3tag, id3BasicFrames[name])
	})

	// Read date frames - try ID3v2.4 first, then fall back to ID3v2.3
	t.Date = getID3JoinedText(id3tag, "TDRC") // ID3v2.4 recording date
	if t.Date == "" {
		// ID3v2.3: combine TYER (year) and TDAT (DDMM) if available
		year := getID3JoinedText(id3tag, "TYER")
		if year != "" {
			t.Date = year
			tdat := getID3JoinedText(id3tag, "TDAT")
			if len(tdat) == 4 {
				// TDAT is DDMM format, convert to YYYY-MM-DD
				day := tdat[0:2]
				month := tdat[2:4]
				t.Date = year + "-" + month + "-" + day
			}
		}
	}

	t.OriginalDate = getID3JoinedText(id3tag, "TDOR") // ID3v2.4 original release date
	if t.OriginalDate == "" {
		// ID3v2.3: TORY is original release year
		tory := getID3JoinedText(id3tag, "TORY")
		if tory != "" {
			t.OriginalDate = tory
		}
	}

	// Original year from TXXX if not found in TDOR/TORY
	if t.OriginalDate == "" {
		origYear := getID3JoinedTXXX(id3tag, "ORIGINALYEAR")
		if origYear != "" {
			t.OriginalDate = origYear
		}
	}

	t.ArtistSortName = getID3JoinedText(id3tag, "TSOP")
	t.Label = getID3JoinedText(id3tag, "TPUB")
	t.Media = getID3JoinedText(id3tag, "TMED")
	t.ISRC = getID3JoinedText(id3tag, "TSRC")

	// Read TXXX (user-defined) frames
	t.MBArtistID = getID3JoinedTXXX(id3tag, "MusicBrainz Artist Id")
	t.MBReleaseID = getID3JoinedTXXX(id3tag, "MusicBrainz Album Id")
	t.MBReleaseGroupID = getID3JoinedTXXX(id3tag, "MusicBrainz Release Group Id")
	t.MBTrackID = getID3JoinedTXXX(id3tag, "MusicBrainz Release Track Id")
	t.CatalogNumber = getID3JoinedTXXX(id3tag, "CATALOGNUMBER")
	t.Barcode = getID3JoinedTXXX(id3tag, "BARCODE")
	t.ReleaseStatus = getID3JoinedTXXX(id3tag, "MusicBrainz Album Status")
	t.ReleaseType = getID3JoinedTXXX(id3tag, "MusicBrainz Album Type")
	t.Script = getID3JoinedTXXX(id3tag, "SCRIPT")
	t.Country = getID3JoinedTXXX(id3tag, "MusicBrainz Album Release Country")

	// Read UFID frame for MusicBrainz Recording ID
	if frames := id3tag.GetFrames("UFID"); len(frames) > 0 {
		for _, frame := range frames {
			if ufid, ok := frame.(id3v2.UFIDFrame); ok {
				if ufid.OwnerIdentifier == "http://musicbrainz.org" {
					t.MBRecordingID = string(ufid.Identifier)
					break
				}
			}
		}
	}
}

// readMP3WithID3v2Fallback reads MP3 metadata using only the id3v2 library.
// This is used as a fallback when dhowden/tag fails (e.g., on some UTF-16 encoded tags).
func readMP3WithID3v2Fallback(path string) (*Tag, error) {
	id3tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return nil, err
	}
	defer id3tag.Close()

	// Parse track number (format: "N" or "N/Total")
	track, totalTracks := parseTrackNumber(firstOf(getID3TextValues(id3tag, "TRCK")))
	disc, totalDiscs := parseTrackNumber(firstOf(getID3TextValues(id3tag, "TPOS")))

	// Parse year from various sources
	date := ""
	if yearStr := id3tag.Year(); yearStr != "" && len(yearStr) >= 4 {
		date = yearStr[:4]
	}

	t := &Tag{
		Path:        path,
		Title:       getID3JoinedText(id3tag, "TIT2"),
		Artist:      getID3JoinedText(id3tag, "TPE1"),
		AlbumArtist: getID3JoinedText(id3tag, "TPE2"),
		Album:       getID3JoinedText(id3tag, "TALB"),
		Date:        date,
		TrackNumber: track,
		TotalTracks: totalTracks,
		DiscNumber:  disc,
		TotalDiscs:  totalDiscs,
		Genre:       getID3JoinedText(id3tag, "TCON"),
	}

	// Read extended tags
	readMP3ExtendedTags(path, t)

	t.applyFallbacks()
	t.Sanitize()
	return t, nil
}

// parseTrackNumber parses a track number string like "5" or "5/10".
func parseTrackNumber(s string) (num, total int) {
	if s == "" {
		return 0, 0
	}
	parts := strings.SplitN(s, "/", 2)
	num, _ = strconv.Atoi(parts[0])
	if len(parts) == 2 {
		total, _ = strconv.Atoi(parts[1])
	}
	return num, total
}

// getID3JoinedText reads the values of a text frame, joined by JoinValues.
func getID3JoinedText(id3tag *id3v2.Tag, frameID string) string {
	return JoinValues(getID3TextValues(id3tag, frameID))
}

// getID3TextValues reads the values of a text frame.
func getID3TextValues(id3tag *id3v2.Tag, frameID string) []string {
	frames := id3tag.GetFrames(frameID)
	if len(frames) == 0 {
		return nil
	}
	if tf, ok := frames[0].(id3v2.TextFrame); ok {
		return splitID3Values(tf.Text)
	}
	return nil
}

// getID3JoinedTXXX reads the values of a user-defined text frame (TXXX),
// joined by JoinValues.
func getID3JoinedTXXX(id3tag *id3v2.Tag, description string) string {
	frames := id3tag.GetFrames("TXXX")
	for _, frame := range frames {
		if txxx, ok := frame.(id3v2.UserDefinedTextFrame); ok {
			if txxx.Description == description {
				return JoinValues(splitID3Values(txxx.Value))
			}
		}
	}
	return ""
}

// splitID3Values splits the text of a frame into its values, which a NUL
// separates in every version of ID3v2. A value in UTF-16 may start with its
// own byte order mark, read as a character.
func splitID3Values(text string) []string {
	values := strings.Split(text, "\x00")
	for i, v := range values {
		values[i] = strings.TrimPrefix(v, "\ufeff")
	}
	return values
}
