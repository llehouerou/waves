package player

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bogem/id3v2/v2"
	"github.com/dhowden/tag"
	goflac "github.com/go-flac/go-flac"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
)

func ReadTrackInfo(path string) (*TrackInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil, err
	}

	title := m.Title()
	if title == "" {
		title = filepath.Base(path)
	}

	track, totalTracks := m.Track()
	disc, totalDiscs := m.Disc()

	albumArtist := m.AlbumArtist()
	if albumArtist == "" {
		albumArtist = m.Artist()
	}

	info := &TrackInfo{
		Path:        path,
		Title:       title,
		Artist:      m.Artist(),
		AlbumArtist: albumArtist,
		Album:       m.Album(),
		Year:        m.Year(),
		Track:       track,
		TotalTracks: totalTracks,
		Disc:        disc,
		TotalDiscs:  totalDiscs,
		Genre:       m.Genre(),
	}

	// Read extended tags based on file format
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case extMP3:
		readMP3ExtendedTags(path, info)
	case extFLAC:
		readFLACExtendedTags(path, info)
	}

	return info, nil
}

// readMP3ExtendedTags reads extended ID3v2 tags from an MP3 file.
func readMP3ExtendedTags(path string, info *TrackInfo) {
	id3tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return
	}
	defer id3tag.Close()

	// Read date frames - try ID3v2.4 first, then fall back to ID3v2.3
	info.Date = getID3TextFrame(id3tag, "TDRC") // ID3v2.4 recording date
	if info.Date == "" {
		// ID3v2.3: combine TYER (year) and TDAT (DDMM) if available
		year := getID3TextFrame(id3tag, "TYER")
		if year != "" {
			info.Date = year
			tdat := getID3TextFrame(id3tag, "TDAT")
			if len(tdat) == 4 {
				// TDAT is DDMM format, convert to YYYY-MM-DD
				day := tdat[0:2]
				month := tdat[2:4]
				info.Date = year + "-" + month + "-" + day
			}
		}
	}

	info.OriginalDate = getID3TextFrame(id3tag, "TDOR") // ID3v2.4 original release date
	if info.OriginalDate == "" {
		// ID3v2.3: TORY is original release year
		tory := getID3TextFrame(id3tag, "TORY")
		if tory != "" {
			info.OriginalDate = tory
		}
	}

	if info.OriginalDate != "" && len(info.OriginalDate) >= 4 {
		info.OriginalYear = info.OriginalDate[:4]
	}
	info.ArtistSortName = getID3TextFrame(id3tag, "TSOP")
	info.Label = getID3TextFrame(id3tag, "TPUB")
	info.Media = getID3TextFrame(id3tag, "TMED")
	info.ISRC = getID3TextFrame(id3tag, "TSRC")

	// Read TXXX (user-defined) frames
	info.MBArtistID = getID3TXXXFrame(id3tag, "MusicBrainz Artist Id")
	info.MBReleaseID = getID3TXXXFrame(id3tag, "MusicBrainz Album Id")
	info.MBReleaseGroupID = getID3TXXXFrame(id3tag, "MusicBrainz Release Group Id")
	info.MBTrackID = getID3TXXXFrame(id3tag, "MusicBrainz Release Track Id")
	info.CatalogNumber = getID3TXXXFrame(id3tag, "CATALOGNUMBER")
	info.Barcode = getID3TXXXFrame(id3tag, "BARCODE")
	info.ReleaseStatus = getID3TXXXFrame(id3tag, "MusicBrainz Album Status")
	info.ReleaseType = getID3TXXXFrame(id3tag, "MusicBrainz Album Type")
	info.Script = getID3TXXXFrame(id3tag, "SCRIPT")
	info.Country = getID3TXXXFrame(id3tag, "MusicBrainz Album Release Country")

	// Original year from TXXX if not found in TDOR/TORY
	if info.OriginalYear == "" {
		info.OriginalYear = getID3TXXXFrame(id3tag, "ORIGINALYEAR")
		if info.OriginalYear != "" && info.OriginalDate == "" {
			info.OriginalDate = info.OriginalYear
		}
	}

	// Read UFID frame for MusicBrainz Recording ID
	if frames := id3tag.GetFrames("UFID"); len(frames) > 0 {
		for _, frame := range frames {
			if ufid, ok := frame.(id3v2.UFIDFrame); ok {
				if ufid.OwnerIdentifier == "http://musicbrainz.org" {
					info.MBRecordingID = string(ufid.Identifier)
					break
				}
			}
		}
	}
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

// readFLACExtendedTags reads extended Vorbis comments from a FLAC file.
func readFLACExtendedTags(path string, info *TrackInfo) {
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
	info.Date = comments["DATE"]
	if info.Date == "" {
		// Fallback to YEAR if DATE not present
		info.Date = comments["YEAR"]
	}
	info.OriginalDate = comments["ORIGINALDATE"]
	info.OriginalYear = comments["ORIGINALYEAR"]
	if info.OriginalYear == "" && info.OriginalDate != "" && len(info.OriginalDate) >= 4 {
		info.OriginalYear = info.OriginalDate[:4]
	}
	info.ArtistSortName = comments["ARTISTSORT"]
	info.Label = comments["LABEL"]
	info.CatalogNumber = comments["CATALOGNUMBER"]
	info.Barcode = comments["BARCODE"]
	info.Media = comments["MEDIA"]
	info.ReleaseStatus = comments["RELEASESTATUS"]
	info.ReleaseType = comments["RELEASETYPE"]
	info.Script = comments["SCRIPT"]
	info.Country = comments["RELEASECOUNTRY"]
	info.ISRC = comments["ISRC"]

	// MusicBrainz IDs
	info.MBArtistID = comments["MUSICBRAINZ_ARTISTID"]
	info.MBReleaseID = comments["MUSICBRAINZ_ALBUMID"]
	info.MBReleaseGroupID = comments["MUSICBRAINZ_RELEASEGROUPID"]
	info.MBRecordingID = comments["MUSICBRAINZ_TRACKID"]
	info.MBTrackID = comments["MUSICBRAINZ_RELEASETRACKID"]
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

// ExtractFullMetadata reads both tag metadata and audio duration.
// It decodes the audio file to determine duration.
func ExtractFullMetadata(path string) (*TrackInfo, error) {
	// First get tag metadata
	info, err := ReadTrackInfo(path)
	if err != nil {
		// If tag reading fails, create basic info from filename
		info = &TrackInfo{
			Path:  path,
			Title: filepath.Base(path),
		}
	}

	// Now decode audio to get duration
	duration, err := getAudioDuration(path)
	if err != nil {
		return nil, err
	}
	info.Duration = duration

	return info, nil
}

func getAudioDuration(path string) (time.Duration, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != extMP3 && ext != extFLAC {
		return 0, fmt.Errorf("unsupported format: %s", ext)
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var streamer beep.StreamSeekCloser
	var format beep.Format

	switch ext {
	case extMP3:
		streamer, format, err = decodeGoMP3(f)
	case extFLAC:
		if err := skipID3v2(f); err != nil {
			return 0, err
		}
		streamer, format, err = flac.Decode(f)
	}
	if err != nil {
		return 0, err
	}
	defer streamer.Close()

	return format.SampleRate.D(streamer.Len()), nil
}

func IsMusicFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == extMP3 || ext == extFLAC
}

// ExtractCoverArt reads cover art for an audio file.
// It first tries to extract embedded art from the file metadata.
// If no embedded art is found, it looks for common cover image files
// in the same directory (cover.jpg, folder.jpg, album.png, etc.).
// Returns the image data and MIME type, or nil if no art is found.
func ExtractCoverArt(path string) (data []byte, mimeType string, err error) {
	// Try embedded art first
	data, mimeType, err = extractEmbeddedArt(path)
	if err != nil {
		return nil, "", err
	}
	if data != nil {
		return data, mimeType, nil
	}

	// Fall back to folder images
	return findFolderArt(filepath.Dir(path))
}

// extractEmbeddedArt reads embedded cover art from an audio file's metadata.
func extractEmbeddedArt(path string) (data []byte, mimeType string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil, "", err
	}

	pic := m.Picture()
	if pic == nil {
		return nil, "", nil
	}

	return pic.Data, pic.MIMEType, nil
}

// Common cover art filenames to look for in album folders.
var coverArtFilenames = []string{
	"cover.jpg", "cover.jpeg", "cover.png",
	"folder.jpg", "folder.jpeg", "folder.png",
	"album.jpg", "album.jpeg", "album.png",
	"front.jpg", "front.jpeg", "front.png",
	"artwork.jpg", "artwork.jpeg", "artwork.png",
}

// findFolderArt looks for common cover art files in the given directory.
func findFolderArt(dir string) (data []byte, mimeType string, err error) {
	for _, filename := range coverArtFilenames {
		imgPath := filepath.Join(dir, filename)
		data, err := os.ReadFile(imgPath)
		if err != nil {
			// Try case-insensitive match
			imgPath = filepath.Join(dir, strings.ToUpper(filename))
			data, err = os.ReadFile(imgPath)
			if err != nil {
				continue
			}
		}

		// Determine MIME type from extension
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".png":
			mimeType = "image/png"
		default:
			mimeType = "application/octet-stream"
		}

		return data, mimeType, nil
	}

	return nil, "", nil
}
