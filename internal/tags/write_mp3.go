package tags

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/bogem/id3v2/v2"
	dhowden "github.com/dhowden/tag"
)

// writeMP3Tags writes ID3v2 tags to an MP3 file.
func writeMP3Tags(path string, t *Tag) error {
	// Told by its header rather than by id3v2.Open, which would leave the
	// file open: its rename would then be refused on Windows.
	v22, err := hasID3v22Tag(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	if v22 {
		return replaceID3v22Tag(path, t)
	}

	// Open the file for tag editing
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer tag.Close()

	setMP3Frames(tag, t, nil)

	// Save changes
	if err := tag.Save(); err != nil {
		return fmt.Errorf("save tags: %w", err)
	}

	return nil
}

// hasID3v22Tag reports whether the file at path starts with an ID3v2 tag older
// than ID3v2.3, which the id3v2 library cannot edit.
func hasID3v22Tag(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	header := make([]byte, 4)
	if _, err := io.ReadFull(f, header); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	return string(header[:3]) == id3Magic && header[3] < 3, nil
}

// replaceID3v22Tag writes the ID3v2.4 tag of t on the MP3 at path in place of
// its ID3v2.2 tag, which the id3v2 library cannot edit. With no new cover, the
// cover of the old tag goes into the new one. The new file replaces the MP3
// whole (replaceFile).
func replaceID3v22Tag(path string, t *Tag) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	audio, err := stripID3v2Tag(data)
	if err != nil {
		return fmt.Errorf("strip unsupported ID3v2.2 tag: %w", err)
	}
	// What follows may be another tag: it is replaced too.
	tag, err := id3v2.ParseReader(bytes.NewReader(audio), id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("parse file after ID3v2.2 tag: %w", err)
	}
	if audio, err = stripID3v2Tag(audio); err != nil {
		return fmt.Errorf("strip ID3v2 tag: %w", err)
	}

	var v22Cover *id3v2.PictureFrame
	if len(t.CoverArt) == 0 {
		v22Cover = readID3v22Cover(data)
	}
	setMP3Frames(tag, t, v22Cover)

	if err := replaceFile(path, buildByWriting(func(w io.Writer) error {
		if _, err := tag.WriteTo(w); err != nil {
			return err
		}
		_, err := w.Write(audio)
		return err
	})); err != nil {
		return fmt.Errorf("save tags: %w", err)
	}

	return nil
}

// setMP3Frames replaces the frames of tag by those of t, as ID3v2.4. With no
// cover in t, the pictures of tag stay, and v22Cover is added when not nil.
func setMP3Frames(tag *id3v2.Tag, t *Tag, v22Cover *id3v2.PictureFrame) {
	// Use ID3v2.4 with UTF-8 for better Unicode support
	tag.SetVersion(4)
	tag.SetDefaultEncoding(id3v2.EncodingUTF8)

	// Clear existing tags to avoid duplicates, keeping the pictures for when
	// no new cover replaces them.
	pictures := tag.GetFrames(tag.CommonID("Attached picture"))
	if v22Cover != nil {
		pictures = append(pictures, *v22Cover)
	}
	tag.DeleteAllFrames()

	// Set basic tags
	tag.SetArtist(t.Artist)
	tag.SetAlbum(t.Album)
	tag.SetTitle(t.Title)
	tag.SetGenre(t.Genre)

	// Set date (TDRC for ID3v2.4 - recording date)
	if t.Date != "" {
		tag.AddTextFrame("TDRC", id3v2.EncodingUTF8, t.Date)
	}

	// Set track number (format: "track/total")
	trackStr := strconv.Itoa(t.TrackNumber)
	if t.TotalTracks > 0 {
		trackStr = strconv.Itoa(t.TrackNumber) + "/" + strconv.Itoa(t.TotalTracks)
	}
	tag.AddTextFrame(tag.CommonID("Track number/Position in set"), id3v2.EncodingUTF8, trackStr)

	// Set disc number (TPOS frame)
	if t.DiscNumber > 0 {
		discStr := strconv.Itoa(t.DiscNumber)
		if t.TotalDiscs > 0 {
			discStr = strconv.Itoa(t.DiscNumber) + "/" + strconv.Itoa(t.TotalDiscs)
		}
		tag.AddTextFrame(tag.CommonID("Part of a set"), id3v2.EncodingUTF8, discStr)
	}

	// Set album artist (TPE2 frame)
	if t.AlbumArtist != "" {
		tag.AddTextFrame(tag.CommonID("Band/Orchestra/Accompaniment"), id3v2.EncodingUTF8, t.AlbumArtist)
	}

	// Set artist sort name (TSOP frame)
	if t.ArtistSortName != "" {
		tag.AddTextFrame("TSOP", id3v2.EncodingUTF8, t.ArtistSortName)
	}

	// Set original date (TDOR frame for ID3v2.4)
	if t.OriginalDate != "" {
		tag.AddTextFrame("TDOR", id3v2.EncodingUTF8, t.OriginalDate)
		// Also add original year as TXXX for broader compatibility
		if len(t.OriginalDate) >= 4 {
			addTXXXFrame(tag, "ORIGINALYEAR", t.OriginalDate[:4])
		}
	}

	// Set label/publisher (TPUB frame)
	if t.Label != "" {
		tag.AddTextFrame("TPUB", id3v2.EncodingUTF8, t.Label)
	}

	// Set media type (TMED frame)
	if t.Media != "" {
		tag.AddTextFrame("TMED", id3v2.EncodingUTF8, t.Media)
	}

	// Set ISRC (TSRC frame)
	if t.ISRC != "" {
		tag.AddTextFrame("TSRC", id3v2.EncodingUTF8, t.ISRC)
	}

	// Set MusicBrainz IDs as TXXX frames (matching Picard's exact descriptions)
	addTXXXFrame(tag, "MusicBrainz Artist Id", t.MBArtistID)
	addTXXXFrame(tag, "MusicBrainz Album Id", t.MBReleaseID)
	addTXXXFrame(tag, "MusicBrainz Release Group Id", t.MBReleaseGroupID)
	addTXXXFrame(tag, "MusicBrainz Release Track Id", t.MBTrackID)

	// Recording ID uses UFID frame in ID3v2.4 (Picard standard)
	if t.MBRecordingID != "" {
		tag.AddFrame("UFID", id3v2.UFIDFrame{
			OwnerIdentifier: "http://musicbrainz.org",
			Identifier:      []byte(t.MBRecordingID),
		})
	}

	// Set other TXXX frames for Picard compatibility
	addTXXXFrame(tag, "CATALOGNUMBER", t.CatalogNumber)
	addTXXXFrame(tag, "BARCODE", t.Barcode)
	addTXXXFrame(tag, "MusicBrainz Album Status", t.ReleaseStatus)
	addTXXXFrame(tag, "MusicBrainz Album Type", t.ReleaseType)
	addTXXXFrame(tag, "SCRIPT", t.Script)
	addTXXXFrame(tag, "MusicBrainz Album Release Country", t.Country)

	// A new cover replaces every picture; without one, the pictures stay.
	if len(t.CoverArt) == 0 {
		for _, pic := range pictures {
			tag.AddFrame(tag.CommonID("Attached picture"), pic)
		}
	} else {
		tag.AddAttachedPicture(frontCover(t.CoverArt, detectMimeType(t.CoverArt), "Front Cover"))
	}
}

// readID3v22Cover reads the cover of the ID3v2.2 tag of data, the content of
// an MP3, which the id3v2
// library cannot parse, as a front cover picture with its description. It
// returns nil when the tag holds no picture, and when dhowden/tag cannot read
// the tag: the picture then goes with the tag the write replaces, which is
// what a write did before waves kept the cover, rather than failing the write.
func readID3v22Cover(data []byte) *id3v2.PictureFrame {
	m, err := dhowden.ReadFrom(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	pic := m.Picture()
	if pic == nil {
		return nil
	}
	cover := frontCover(pic.Data, id3v22MimeType(pic), pic.Description)
	return &cover
}

// id3v22MimeType returns the MIME type of a picture read from an ID3v2.2 tag,
// which names its image format ("JPG", "GIF") instead.
func id3v22MimeType(pic *dhowden.Picture) string {
	if pic.MIMEType != "" {
		return pic.MIMEType
	}
	switch format := strings.ToLower(pic.Ext); format {
	case "":
		return detectMimeType(pic.Data)
	case "jpg":
		return mimeJPEG
	default:
		return "image/" + format
	}
}

// frontCover is an attached picture holding data as the front cover.
func frontCover(data []byte, mimeType, description string) id3v2.PictureFrame {
	return id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    mimeType,
		PictureType: id3v2.PTFrontCover,
		Description: description,
		Picture:     data,
	}
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

// stripID3v2Tag returns the content of an MP3 file without the ID3v2 tag it
// starts with, or the content itself when it starts with none.
func stripID3v2Tag(data []byte) ([]byte, error) {
	// Check for ID3v2 header (must have at least 10 bytes for header)
	if len(data) < 10 || string(data[:3]) != id3Magic {
		return data, nil // No ID3v2 tag to strip
	}

	// Parse tag size from bytes 6-9 (synchsafe integer: each byte uses only 7 bits)
	size := int(data[6])<<21 | int(data[7])<<14 | int(data[8])<<7 | int(data[9])
	tagSize := size + 10 // Add 10-byte header

	// Check for footer flag (bit 4 of flags byte) - ID3v2.4 only
	if data[5]&0x10 != 0 {
		tagSize += 10
	}

	if tagSize >= len(data) {
		return nil, fmt.Errorf("ID3v2 tag size (%d) exceeds file size (%d)", tagSize, len(data))
	}

	return data[tagSize:], nil
}
