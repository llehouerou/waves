package importer

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/bogem/id3v2/v2"
)

const mimeJPEG = "image/jpeg"

// writeMP3Tags writes ID3v2 tags to an MP3 file.
func writeMP3Tags(path string, data TagData) error {
	// Open the file for tag editing
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer tag.Close()

	// Use ID3v2.4 with UTF-8 for better Unicode support
	tag.SetVersion(4)
	tag.SetDefaultEncoding(id3v2.EncodingUTF8)

	// Clear existing tags to avoid duplicates
	tag.DeleteAllFrames()

	// Set basic tags
	tag.SetArtist(data.Artist)
	tag.SetAlbum(data.Album)
	tag.SetTitle(data.Title)
	tag.SetGenre(data.Genre)

	// Set date (TDRC for ID3v2.4 - recording date)
	if data.Date != "" {
		tag.AddTextFrame("TDRC", id3v2.EncodingUTF8, data.Date)
	}

	// Set track number (format: "track/total")
	trackStr := strconv.Itoa(data.TrackNumber)
	if data.TotalTracks > 0 {
		trackStr = strconv.Itoa(data.TrackNumber) + "/" + strconv.Itoa(data.TotalTracks)
	}
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), id3v2.EncodingUTF8, trackStr)

	// Set disc number (TPOS frame)
	if data.DiscNumber > 0 {
		discStr := strconv.Itoa(data.DiscNumber)
		if data.TotalDiscs > 0 {
			discStr = strconv.Itoa(data.DiscNumber) + "/" + strconv.Itoa(data.TotalDiscs)
		}
		tag.AddTextFrame(tag.CommonID("Part of a set"), id3v2.EncodingUTF8, discStr)
	}

	// Set album artist (TPE2 frame)
	if data.AlbumArtist != "" {
		tag.AddTextFrame(tag.CommonID("Band/Orchestra/Accompaniment"), id3v2.EncodingUTF8, data.AlbumArtist)
	}

	// Set artist sort name (TSOP frame)
	if data.ArtistSortName != "" {
		tag.AddTextFrame("TSOP", id3v2.EncodingUTF8, data.ArtistSortName)
	}

	// Set original date (TDOR frame for ID3v2.4)
	if data.OriginalDate != "" {
		tag.AddTextFrame("TDOR", id3v2.EncodingUTF8, data.OriginalDate)
		// Also add original year as TXXX for broader compatibility
		if len(data.OriginalDate) >= 4 {
			addTXXXFrame(tag, "ORIGINALYEAR", data.OriginalDate[:4])
		}
	}

	// Set label/publisher (TPUB frame)
	if data.Label != "" {
		tag.AddTextFrame("TPUB", id3v2.EncodingUTF8, data.Label)
	}

	// Set media type (TMED frame)
	if data.Media != "" {
		tag.AddTextFrame("TMED", id3v2.EncodingUTF8, data.Media)
	}

	// Set ISRC (TSRC frame)
	if data.ISRC != "" {
		tag.AddTextFrame("TSRC", id3v2.EncodingUTF8, data.ISRC)
	}

	// Set MusicBrainz IDs as TXXX frames (matching Picard's exact descriptions)
	addTXXXFrame(tag, "MusicBrainz Artist Id", data.MBArtistID)
	addTXXXFrame(tag, "MusicBrainz Album Id", data.MBReleaseID)
	addTXXXFrame(tag, "MusicBrainz Release Group Id", data.MBReleaseGroupID)
	addTXXXFrame(tag, "MusicBrainz Release Track Id", data.MBTrackID)

	// Recording ID uses UFID frame in ID3v2.4 (Picard standard)
	if data.MBRecordingID != "" {
		tag.AddFrame("UFID", id3v2.UFIDFrame{
			OwnerIdentifier: "http://musicbrainz.org",
			Identifier:      []byte(data.MBRecordingID),
		})
	}

	// Set other TXXX frames for Picard compatibility
	addTXXXFrame(tag, "CATALOGNUMBER", data.CatalogNumber)
	addTXXXFrame(tag, "BARCODE", data.Barcode)
	addTXXXFrame(tag, "MusicBrainz Album Status", data.ReleaseStatus)
	addTXXXFrame(tag, "MusicBrainz Album Type", data.ReleaseType)
	addTXXXFrame(tag, "SCRIPT", data.Script)
	addTXXXFrame(tag, "MusicBrainz Album Release Country", data.Country)

	// Add cover art if provided
	if len(data.CoverArt) > 0 {
		mimeType := detectMimeType(data.CoverArt)
		pic := id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    mimeType,
			PictureType: id3v2.PTFrontCover,
			Description: "Front Cover",
			Picture:     data.CoverArt,
		}
		tag.AddAttachedPicture(pic)
	}

	// Save changes
	if err := tag.Save(); err != nil {
		return fmt.Errorf("save tags: %w", err)
	}

	return nil
}

// addTXXXFrame adds a TXXX (user-defined text) frame if the value is non-empty.
func addTXXXFrame(tag *id3v2.Tag, description, value string) {
	if value == "" {
		return
	}
	tag.AddUserDefinedTextFrame(id3v2.UserDefinedTextFrame{
		Encoding:    id3v2.EncodingUTF8,
		Description: description,
		Value:       value,
	})
}

// detectMimeType detects the MIME type of image data.
func detectMimeType(data []byte) string {
	if len(data) == 0 {
		return mimeJPEG
	}
	contentType := http.DetectContentType(data)
	// http.DetectContentType may return more specific types, normalize to common ones
	switch contentType {
	case mimeJPEG:
		return mimeJPEG
	case "image/png":
		return "image/png"
	default:
		// Default to JPEG for unknown types
		return mimeJPEG
	}
}
