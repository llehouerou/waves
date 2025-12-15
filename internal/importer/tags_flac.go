package importer

import (
	"fmt"
	"strconv"

	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
)

// writeFLACTags writes Vorbis comments and picture to a FLAC file.
func writeFLACTags(path string, data TagData) error {
	// Parse the FLAC file
	f, err := flac.ParseFile(path)
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
	// This replaces any existing comments with our new ones
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
	if err := addTag("ARTIST", data.Artist); err != nil {
		return fmt.Errorf("add artist: %w", err)
	}
	if err := addTag("ALBUMARTIST", data.AlbumArtist); err != nil {
		return fmt.Errorf("add album artist: %w", err)
	}
	if err := addTag("ALBUM", data.Album); err != nil {
		return fmt.Errorf("add album: %w", err)
	}
	if err := addTag("TITLE", data.Title); err != nil {
		return fmt.Errorf("add title: %w", err)
	}
	if err := addTag("GENRE", data.Genre); err != nil {
		return fmt.Errorf("add genre: %w", err)
	}

	// Track/disc numbers
	if err := addIntTag("TRACKNUMBER", data.TrackNumber); err != nil {
		return fmt.Errorf("add track number: %w", err)
	}
	if err := addIntTag("TOTALTRACKS", data.TotalTracks); err != nil {
		return fmt.Errorf("add total tracks: %w", err)
	}
	if err := addIntTag("DISCNUMBER", data.DiscNumber); err != nil {
		return fmt.Errorf("add disc number: %w", err)
	}
	if err := addIntTag("TOTALDISCS", data.TotalDiscs); err != nil {
		return fmt.Errorf("add total discs: %w", err)
	}

	// Date tags
	if err := addTag("DATE", data.Date); err != nil {
		return fmt.Errorf("add date: %w", err)
	}
	if err := addTag("ORIGINALDATE", data.OriginalDate); err != nil {
		return fmt.Errorf("add original date: %w", err)
	}
	// ORIGINALYEAR is just the year portion of ORIGINALDATE
	if data.OriginalDate != "" && len(data.OriginalDate) >= 4 {
		if err := addTag("ORIGINALYEAR", data.OriginalDate[:4]); err != nil {
			return fmt.Errorf("add original year: %w", err)
		}
	}

	// Artist info
	if err := addTag("ARTISTSORT", data.ArtistSortName); err != nil {
		return fmt.Errorf("add artist sort: %w", err)
	}

	// Release info
	if err := addTag("LABEL", data.Label); err != nil {
		return fmt.Errorf("add label: %w", err)
	}
	if err := addTag("CATALOGNUMBER", data.CatalogNumber); err != nil {
		return fmt.Errorf("add catalog number: %w", err)
	}
	if err := addTag("BARCODE", data.Barcode); err != nil {
		return fmt.Errorf("add barcode: %w", err)
	}
	if err := addTag("MEDIA", data.Media); err != nil {
		return fmt.Errorf("add media: %w", err)
	}
	if err := addTag("RELEASESTATUS", data.ReleaseStatus); err != nil {
		return fmt.Errorf("add release status: %w", err)
	}
	if err := addTag("RELEASETYPE", data.ReleaseType); err != nil {
		return fmt.Errorf("add release type: %w", err)
	}
	if err := addTag("SCRIPT", data.Script); err != nil {
		return fmt.Errorf("add script: %w", err)
	}
	if err := addTag("RELEASECOUNTRY", data.Country); err != nil {
		return fmt.Errorf("add country: %w", err)
	}

	// MusicBrainz IDs
	if err := addTag("MUSICBRAINZ_ARTISTID", data.MBArtistID); err != nil {
		return fmt.Errorf("add mb artist id: %w", err)
	}
	if err := addTag("MUSICBRAINZ_ALBUMID", data.MBReleaseID); err != nil {
		return fmt.Errorf("add mb release id: %w", err)
	}
	if err := addTag("MUSICBRAINZ_RELEASEGROUPID", data.MBReleaseGroupID); err != nil {
		return fmt.Errorf("add mb release group id: %w", err)
	}
	if err := addTag("MUSICBRAINZ_RELEASETRACKID", data.MBTrackID); err != nil {
		return fmt.Errorf("add mb track id: %w", err)
	}
	if err := addTag("MUSICBRAINZ_TRACKID", data.MBRecordingID); err != nil {
		return fmt.Errorf("add mb recording id: %w", err)
	}

	// Recording info
	if err := addTag("ISRC", data.ISRC); err != nil {
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
	if len(data.CoverArt) > 0 {
		// Remove existing picture blocks
		newMeta := make([]*flac.MetaDataBlock, 0, len(f.Meta))
		for _, meta := range f.Meta {
			if meta.Type != flac.Picture {
				newMeta = append(newMeta, meta)
			}
		}
		f.Meta = newMeta

		// Create picture block
		mimeType := detectMimeType(data.CoverArt)
		pic, err := flacpicture.NewFromImageData(
			flacpicture.PictureTypeFrontCover,
			"Front Cover",
			data.CoverArt,
			mimeType,
		)
		if err != nil {
			return fmt.Errorf("create picture: %w", err)
		}

		picBlock := pic.Marshal()
		f.Meta = append(f.Meta, &picBlock)
	}

	// Save the file
	if err := f.Save(path); err != nil {
		return fmt.Errorf("save file: %w", err)
	}

	return nil
}
