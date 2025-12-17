package importer

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/bogem/id3v2/v2"
)

const mimeJPEG = "image/jpeg"

// writeMP3Tags writes ID3v2 tags to an MP3 file.
func writeMP3Tags(path string, data TagData) error {
	// Open the file for tag editing
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if errors.Is(err, id3v2.ErrUnsupportedVersion) {
		// ID3v2.2 or older tags - strip them and retry
		if stripErr := stripID3v2Tag(path); stripErr != nil {
			return fmt.Errorf("strip unsupported ID3v2.2 tag: %w", stripErr)
		}
		tag, err = id3v2.Open(path, id3v2.Options{Parse: true})
	}
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

// stripID3v2Tag removes ID3v2 tags from an MP3 file.
// This is used to handle ID3v2.2 tags which the id3v2 library doesn't support.
func stripID3v2Tag(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Check for ID3v2 header (must have at least 10 bytes for header)
	if len(data) < 10 || string(data[:3]) != "ID3" {
		return nil // No ID3v2 tag to strip
	}

	// Parse tag size from bytes 6-9 (synchsafe integer: each byte uses only 7 bits)
	size := int(data[6])<<21 | int(data[7])<<14 | int(data[8])<<7 | int(data[9])
	tagSize := size + 10 // Add 10-byte header

	// Check for footer flag (bit 4 of flags byte) - ID3v2.4 only
	if data[5]&0x10 != 0 {
		tagSize += 10
	}

	if tagSize >= len(data) {
		return fmt.Errorf("ID3v2 tag size (%d) exceeds file size (%d)", tagSize, len(data))
	}

	// Preserve original file permissions
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	// Write audio data without the ID3v2 tag
	if err := os.WriteFile(path, data[tagSize:], info.Mode()); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}
