package tags

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
)

// writeFLACTags writes Vorbis comments and picture to a FLAC file. The new
// file replaces it whole (replaceFile), without the ID3v2 header the file may
// start with.
func writeFLACTags(path string, t *Tag) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	f, err := parseFLACWithID3Support(data)
	if err != nil {
		return fmt.Errorf("parse file: %w", err)
	}

	// Find existing VORBIS_COMMENT block index (if any)
	cmtIdx := -1
	for i, meta := range f.Meta {
		if meta.Type == flac.VorbisComment {
			cmtIdx = i
			break
		}
	}

	// Always create a fresh comment block to avoid duplicate tags
	cmts := flacvorbis.New()

	// Helper to add tag if non-empty
	addTag := func(key, value string) error {
		if value != "" {
			return cmts.Add(key, value)
		}
		return nil
	}

	// Helper to add int tag if > 0
	addIntTag := func(key string, value int) error {
		if value > 0 {
			return cmts.Add(key, strconv.Itoa(value))
		}
		return nil
	}

	// Basic tags
	if err := addTag("ARTIST", t.Artist); err != nil {
		return fmt.Errorf("add artist: %w", err)
	}
	if err := addTag("ALBUMARTIST", t.AlbumArtist); err != nil {
		return fmt.Errorf("add album artist: %w", err)
	}
	if err := addTag("ALBUM", t.Album); err != nil {
		return fmt.Errorf("add album: %w", err)
	}
	if err := addTag("TITLE", t.Title); err != nil {
		return fmt.Errorf("add title: %w", err)
	}
	if err := addTag("GENRE", t.Genre); err != nil {
		return fmt.Errorf("add genre: %w", err)
	}

	// Track/disc numbers
	if err := addIntTag("TRACKNUMBER", t.TrackNumber); err != nil {
		return fmt.Errorf("add track number: %w", err)
	}
	if err := addIntTag("TOTALTRACKS", t.TotalTracks); err != nil {
		return fmt.Errorf("add total tracks: %w", err)
	}
	if err := addIntTag("DISCNUMBER", t.DiscNumber); err != nil {
		return fmt.Errorf("add disc number: %w", err)
	}
	if err := addIntTag("TOTALDISCS", t.TotalDiscs); err != nil {
		return fmt.Errorf("add total discs: %w", err)
	}

	// Date tags
	if err := addTag("DATE", t.Date); err != nil {
		return fmt.Errorf("add date: %w", err)
	}
	if err := addTag("ORIGINALDATE", t.OriginalDate); err != nil {
		return fmt.Errorf("add original date: %w", err)
	}
	// ORIGINALYEAR is just the year portion of ORIGINALDATE
	if t.OriginalDate != "" && len(t.OriginalDate) >= 4 {
		if err := addTag("ORIGINALYEAR", t.OriginalDate[:4]); err != nil {
			return fmt.Errorf("add original year: %w", err)
		}
	}

	// Artist info
	if err := addTag("ARTISTSORT", t.ArtistSortName); err != nil {
		return fmt.Errorf("add artist sort: %w", err)
	}

	// Release info
	if err := addTag("LABEL", t.Label); err != nil {
		return fmt.Errorf("add label: %w", err)
	}
	if err := addTag("CATALOGNUMBER", t.CatalogNumber); err != nil {
		return fmt.Errorf("add catalog number: %w", err)
	}
	if err := addTag("BARCODE", t.Barcode); err != nil {
		return fmt.Errorf("add barcode: %w", err)
	}
	if err := addTag("MEDIA", t.Media); err != nil {
		return fmt.Errorf("add media: %w", err)
	}
	if err := addTag("RELEASESTATUS", t.ReleaseStatus); err != nil {
		return fmt.Errorf("add release status: %w", err)
	}
	if err := addTag("RELEASETYPE", t.ReleaseType); err != nil {
		return fmt.Errorf("add release type: %w", err)
	}
	if err := addTag("SCRIPT", t.Script); err != nil {
		return fmt.Errorf("add script: %w", err)
	}
	if err := addTag("RELEASECOUNTRY", t.Country); err != nil {
		return fmt.Errorf("add country: %w", err)
	}

	// MusicBrainz IDs
	if err := addTag("MUSICBRAINZ_ARTISTID", t.MBArtistID); err != nil {
		return fmt.Errorf("add mb artist id: %w", err)
	}
	if err := addTag("MUSICBRAINZ_ALBUMID", t.MBReleaseID); err != nil {
		return fmt.Errorf("add mb release id: %w", err)
	}
	if err := addTag("MUSICBRAINZ_RELEASEGROUPID", t.MBReleaseGroupID); err != nil {
		return fmt.Errorf("add mb release group id: %w", err)
	}
	if err := addTag("MUSICBRAINZ_RELEASETRACKID", t.MBTrackID); err != nil {
		return fmt.Errorf("add mb track id: %w", err)
	}
	if err := addTag("MUSICBRAINZ_TRACKID", t.MBRecordingID); err != nil {
		return fmt.Errorf("add mb recording id: %w", err)
	}

	// Recording info
	if err := addTag("ISRC", t.ISRC); err != nil {
		return fmt.Errorf("add isrc: %w", err)
	}

	// Marshal the comment block
	cmtBlock := cmts.Marshal()

	// Update or add the comment block
	if cmtIdx >= 0 {
		f.Meta[cmtIdx] = &cmtBlock
	} else {
		f.Meta = append(f.Meta, &cmtBlock)
	}

	// Add cover art if provided
	if len(t.CoverArt) > 0 {
		// Remove existing picture blocks
		newMeta := make([]*flac.MetaDataBlock, 0, len(f.Meta))
		for _, meta := range f.Meta {
			if meta.Type != flac.Picture {
				newMeta = append(newMeta, meta)
			}
		}
		f.Meta = newMeta

		// Create picture block
		mimeType := detectMimeType(t.CoverArt)
		pic, err := flacpicture.NewFromImageData(
			flacpicture.PictureTypeFrontCover,
			"Front Cover",
			t.CoverArt,
			mimeType,
		)
		if err != nil {
			return fmt.Errorf("create picture: %w", err)
		}

		picBlock := pic.Marshal()
		f.Meta = append(f.Meta, &picBlock)
	}

	if err := replaceFile(path, func(w io.Writer) error {
		_, err := w.Write(f.Marshal())
		return err
	}); err != nil {
		return fmt.Errorf("save file: %w", err)
	}

	return nil
}

// parseFLACWithID3Support parses the content of a FLAC file, past the ID3v2
// header it may start with.
func parseFLACWithID3Support(data []byte) (*flac.File, error) {
	// First try normal parsing
	f, err := flac.ParseBytes(bytes.NewReader(data))
	if err == nil {
		return f, nil
	}

	// Check for ID3v2 header (starts with "ID3")
	if len(data) < 10 || !bytes.Equal(data[:3], []byte(id3Magic)) {
		return nil, err // Not an ID3v2 header, return original error
	}
	header := data[:10]

	// Calculate ID3v2 header size
	// Size is stored in bytes 6-9 as syncsafe integer (7 bits per byte)
	id3Size := int64(10) // Base header size
	id3Size += int64(header[6]&0x7f)<<21 |
		int64(header[7]&0x7f)<<14 |
		int64(header[8]&0x7f)<<7 |
		int64(header[9]&0x7f)

	// Check for extended header flag
	if header[5]&0x40 != 0 {
		// Extended header present, need to read its size too
		if len(data) < 14 {
			return nil, err
		}
		extHeader := data[10:14]
		extSize := int64(extHeader[0]&0x7f)<<21 |
			int64(extHeader[1]&0x7f)<<14 |
			int64(extHeader[2]&0x7f)<<7 |
			int64(extHeader[3]&0x7f)
		id3Size += extSize
	}

	// Verify FLAC magic after ID3v2 header
	if int64(len(data)) < id3Size+4 {
		return nil, err
	}
	if !bytes.Equal(data[id3Size:id3Size+4], []byte("fLaC")) {
		return nil, errors.New("no fLaC marker found after ID3v2 header")
	}

	f, err = flac.ParseBytes(bytes.NewReader(data[id3Size:]))
	if err != nil {
		return nil, fmt.Errorf("parse after ID3v2 header: %w", err)
	}
	return f, nil
}
