package tags

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bogem/id3v2/v2"
)

// readMP3ExtendedTags reads extended ID3v2 tags from an MP3 file.
func readMP3ExtendedTags(path string, t *Tag) {
	id3tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return
	}
	defer id3tag.Close()

	// Read date frames - try ID3v2.4 first, then fall back to ID3v2.3
	t.Date = getID3TextFrame(id3tag, "TDRC") // ID3v2.4 recording date
	if t.Date == "" {
		// ID3v2.3: combine TYER (year) and TDAT (DDMM) if available
		year := getID3TextFrame(id3tag, "TYER")
		if year != "" {
			t.Date = year
			tdat := getID3TextFrame(id3tag, "TDAT")
			if len(tdat) == 4 {
				// TDAT is DDMM format, convert to YYYY-MM-DD
				day := tdat[0:2]
				month := tdat[2:4]
				t.Date = year + "-" + month + "-" + day
			}
		}
	}

	t.OriginalDate = getID3TextFrame(id3tag, "TDOR") // ID3v2.4 original release date
	if t.OriginalDate == "" {
		// ID3v2.3: TORY is original release year
		tory := getID3TextFrame(id3tag, "TORY")
		if tory != "" {
			t.OriginalDate = tory
		}
	}

	// Original year from TXXX if not found in TDOR/TORY
	if t.OriginalDate == "" {
		origYear := getID3TXXXFrame(id3tag, "ORIGINALYEAR")
		if origYear != "" {
			t.OriginalDate = origYear
		}
	}

	t.ArtistSortName = getID3TextFrame(id3tag, "TSOP")
	t.Label = getID3TextFrame(id3tag, "TPUB")
	t.Media = getID3TextFrame(id3tag, "TMED")
	t.ISRC = getID3TextFrame(id3tag, "TSRC")

	// Read TXXX (user-defined) frames
	t.MBArtistID = getID3TXXXFrame(id3tag, "MusicBrainz Artist Id")
	t.MBReleaseID = getID3TXXXFrame(id3tag, "MusicBrainz Album Id")
	t.MBReleaseGroupID = getID3TXXXFrame(id3tag, "MusicBrainz Release Group Id")
	t.MBTrackID = getID3TXXXFrame(id3tag, "MusicBrainz Release Track Id")
	t.CatalogNumber = getID3TXXXFrame(id3tag, "CATALOGNUMBER")
	t.Barcode = getID3TXXXFrame(id3tag, "BARCODE")
	t.ReleaseStatus = getID3TXXXFrame(id3tag, "MusicBrainz Album Status")
	t.ReleaseType = getID3TXXXFrame(id3tag, "MusicBrainz Album Type")
	t.Script = getID3TXXXFrame(id3tag, "SCRIPT")
	t.Country = getID3TXXXFrame(id3tag, "MusicBrainz Album Release Country")

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

	title := id3tag.Title()
	if title == "" {
		title = filepath.Base(path)
	}

	artist := id3tag.Artist()
	albumArtist := getID3TextFrame(id3tag, "TPE2") // Album artist frame
	if albumArtist == "" {
		albumArtist = artist
	}

	// Parse track number (format: "N" or "N/Total")
	track, totalTracks := parseTrackNumber(getID3TextFrame(id3tag, "TRCK"))
	disc, totalDiscs := parseTrackNumber(getID3TextFrame(id3tag, "TPOS"))

	// Parse year from various sources
	date := ""
	if yearStr := id3tag.Year(); yearStr != "" && len(yearStr) >= 4 {
		date = yearStr[:4]
	}

	t := &Tag{
		Path:        path,
		Title:       title,
		Artist:      artist,
		AlbumArtist: albumArtist,
		Album:       id3tag.Album(),
		Date:        date,
		TrackNumber: track,
		TotalTracks: totalTracks,
		DiscNumber:  disc,
		TotalDiscs:  totalDiscs,
		Genre:       id3tag.Genre(),
	}

	// Read extended tags
	readMP3ExtendedTags(path, t)

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

// getID3TextFrame reads a text frame value from an ID3v2 tag.
func getID3TextFrame(id3tag *id3v2.Tag, frameID string) string {
	frames := id3tag.GetFrames(frameID)
	if len(frames) == 0 {
		return ""
	}
	if tf, ok := frames[0].(id3v2.TextFrame); ok {
		return tf.Text
	}
	return ""
}

// getID3TXXXFrame reads a user-defined text frame (TXXX) value.
func getID3TXXXFrame(id3tag *id3v2.Tag, description string) string {
	frames := id3tag.GetFrames("TXXX")
	for _, frame := range frames {
		if txxx, ok := frame.(id3v2.UserDefinedTextFrame); ok {
			if txxx.Description == description {
				return txxx.Value
			}
		}
	}
	return ""
}
