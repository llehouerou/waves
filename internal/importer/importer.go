// Package importer provides functionality to import music files into the library
// with proper tagging and renaming based on MusicBrainz metadata.
package importer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/llehouerou/waves/internal/musicbrainz"
	"github.com/llehouerou/waves/internal/rename"
)

// Retry configuration
const (
	maxRetries       = 3
	initialBackoff   = 500 * time.Millisecond
	maxBackoff       = 5 * time.Second
	operationTimeout = 30 * time.Second
)

// File extensions
const (
	extMP3  = ".mp3"
	extFLAC = ".flac"
	extOPUS = ".opus"
	extOGG  = ".ogg"
	extM4A  = ".m4a"
	extMP4  = ".mp4"
)

// ImportParams contains all the data needed to import a track.
type ImportParams struct {
	SourcePath   string                      // Path to source file
	DestRoot     string                      // Library root directory
	ReleaseGroup *musicbrainz.ReleaseGroup   // For OriginalDate, SecondaryTypes, Genres
	Release      *musicbrainz.ReleaseDetails // For Date, Artist, Album, Tracks, Genres
	TrackIndex   int                         // Index into Release.Tracks (0-based)
	DiscNumber   int                         // 1-based disc number
	TotalDiscs   int                         // Total number of discs
	CoverArt     []byte                      // Optional: pre-fetched cover art (JPEG/PNG)
	CopyMode     bool                        // If true, copy file instead of moving
	RenameConfig rename.Config               // Rename configuration (uses DefaultConfig if empty)
}

// ImportResult contains the result of an import operation.
type ImportResult struct {
	DestPath string // Final path where file was moved
}

// Import imports a music file into the library with proper tags and naming.
// The file is retagged with MusicBrainz metadata, renamed according to the
// naming convention, and moved to the destination directory.
// Operations are retried with exponential backoff on temporary failures.
func Import(p ImportParams) (*ImportResult, error) {
	// Create a context with overall timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Validate inputs
	if err := validateParams(p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// Check source file exists
	if _, err := os.Stat(p.SourcePath); err != nil {
		return nil, fmt.Errorf("source file: %w", err)
	}

	// Detect file format from extension
	ext := strings.ToLower(filepath.Ext(p.SourcePath))
	if ext != extMP3 && ext != extFLAC && ext != extOPUS && ext != extOGG && ext != extM4A && ext != extMP4 {
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}

	// Build metadata for renaming
	track := &p.Release.Tracks[p.TrackIndex]

	// Use track-level artist if set (for featuring artists), otherwise album artist
	trackArtist := track.Artist
	if trackArtist == "" {
		trackArtist = p.Release.Artist
	}

	meta := rename.TrackMetadata{
		Artist:               trackArtist,
		AlbumArtist:          p.Release.Artist,
		Album:                p.Release.Title,
		Title:                track.Title,
		TrackNumber:          track.Position,
		DiscNumber:           p.DiscNumber,
		TotalDiscs:           p.TotalDiscs,
		Date:                 p.Release.Date,
		OriginalDate:         p.ReleaseGroup.FirstRelease,
		ReleaseType:          strings.ToLower(p.ReleaseGroup.PrimaryType),
		SecondaryReleaseType: strings.Join(p.ReleaseGroup.SecondaryTypes, "; "),
	}

	// Generate destination path using provided config or default
	cfg := p.RenameConfig
	if cfg.Folder == "" {
		cfg = rename.DefaultConfig()
	}
	relPath := rename.GeneratePathWithConfig(meta, cfg)
	destPath := filepath.Join(p.DestRoot, relPath+ext)

	// Build tag data using shared helper
	tagData := BuildTagData(BuildTagDataParams{
		ReleaseGroup: p.ReleaseGroup,
		Release:      p.Release,
		Track:        track,
		DiscNumber:   p.DiscNumber,
		TotalDiscs:   p.TotalDiscs,
		CoverArt:     p.CoverArt,
	})

	// Write tags to source file with retry
	switch ext {
	case extMP3:
		err := retryWithBackoff(ctx, "write MP3 tags", func() error {
			return writeMP3Tags(p.SourcePath, tagData)
		})
		if err != nil {
			return nil, err
		}
	case extFLAC:
		err := retryWithBackoff(ctx, "write FLAC tags", func() error {
			return writeFLACTags(p.SourcePath, tagData)
		})
		if err != nil {
			return nil, err
		}
	case extOPUS, extOGG:
		err := retryWithBackoff(ctx, "write Opus tags", func() error {
			return writeOpusTags(p.SourcePath, tagData)
		})
		if err != nil {
			return nil, err
		}
	case extM4A, extMP4:
		err := retryWithBackoff(ctx, "write M4A tags", func() error {
			return writeM4ATags(p.SourcePath, tagData)
		})
		if err != nil {
			return nil, err
		}
	}

	// Create destination directory with retry
	destDir := filepath.Dir(destPath)
	err := retryWithBackoff(ctx, "create directory", func() error {
		return os.MkdirAll(destDir, 0o755)
	})
	if err != nil {
		return nil, err
	}

	// Copy or move file to destination with retry
	if p.CopyMode {
		err := retryWithBackoff(ctx, "copy file", func() error {
			return copyFile(p.SourcePath, destPath)
		})
		if err != nil {
			return nil, err
		}
	} else {
		err := retryWithBackoff(ctx, "move file", func() error {
			return moveFile(p.SourcePath, destPath)
		})
		if err != nil {
			return nil, err
		}
	}

	return &ImportResult{DestPath: destPath}, nil
}

// TagData contains the tag values to write to a file.
type TagData struct {
	// Basic tags
	Artist      string
	AlbumArtist string
	Album       string
	Title       string
	TrackNumber int
	TotalTracks int
	DiscNumber  int
	TotalDiscs  int

	// Date tags
	Date         string // Release date (YYYY-MM-DD or YYYY)
	OriginalDate string // Original release date (YYYY-MM-DD or YYYY)

	// Genre (multiple genres separated by ";")
	Genre string

	// Artist info
	ArtistSortName string

	// Release info
	Label         string
	CatalogNumber string
	Barcode       string
	Media         string // Format (CD, Vinyl, Digital, etc.)
	ReleaseStatus string // Official, Promotional, Bootleg
	ReleaseType   string // Album, Single, EP, etc.
	Script        string // Latn, Cyrl, etc.
	Country       string

	// MusicBrainz IDs
	MBArtistID       string
	MBReleaseID      string
	MBReleaseGroupID string
	MBRecordingID    string
	MBTrackID        string

	// Recording info
	ISRC string // International Standard Recording Code

	// Artwork
	CoverArt []byte // JPEG or PNG image data
}

// BuildTagDataParams contains parameters for building TagData from MusicBrainz metadata.
type BuildTagDataParams struct {
	ReleaseGroup  *musicbrainz.ReleaseGroup
	Release       *musicbrainz.ReleaseDetails
	Track         *musicbrainz.Track // The specific track
	DiscNumber    int
	TotalDiscs    int
	CoverArt      []byte // Optional cover art
	ExistingGenre string // If set and new genre is empty, preserve this
}

// BuildTagData creates TagData from MusicBrainz metadata.
// This is shared between import and retag to ensure consistent tagging.
func BuildTagData(p BuildTagDataParams) TagData {
	// Use track-level artist if set, otherwise album artist
	trackArtist := p.Track.Artist
	if trackArtist == "" {
		trackArtist = p.Release.Artist
	}

	trackArtistID := p.Track.ArtistID
	if trackArtistID == "" {
		trackArtistID = p.Release.ArtistID
	}

	// Build genre string - preserve existing if new is empty
	genre := BuildGenreString(p.Release.Genres, p.ReleaseGroup.Genres)
	if genre == "" && p.ExistingGenre != "" {
		genre = p.ExistingGenre
	}

	// Build release type string
	releaseType := strings.ToLower(p.ReleaseGroup.PrimaryType)
	if len(p.ReleaseGroup.SecondaryTypes) > 0 {
		releaseType += "; " + strings.Join(p.ReleaseGroup.SecondaryTypes, "; ")
	}

	return TagData{
		// Basic tags
		Artist:      trackArtist,
		AlbumArtist: p.Release.Artist,
		Album:       p.Release.Title,
		Title:       p.Track.Title,
		TrackNumber: p.Track.Position,
		TotalTracks: len(p.Release.Tracks),
		DiscNumber:  p.DiscNumber,
		TotalDiscs:  p.TotalDiscs,

		// Date tags
		Date:         p.Release.Date,
		OriginalDate: p.ReleaseGroup.FirstRelease,

		// Genre
		Genre: genre,

		// Artist info
		ArtistSortName: p.Release.ArtistSortName,

		// Release info
		Label:         p.Release.Label,
		CatalogNumber: p.Release.CatalogNumber,
		Barcode:       p.Release.Barcode,
		Media:         p.Release.Formats,
		ReleaseStatus: p.Release.Status,
		ReleaseType:   releaseType,
		Script:        p.Release.Script,
		Country:       p.Release.Country,

		// MusicBrainz IDs
		MBArtistID:       trackArtistID,
		MBReleaseID:      p.Release.ID,
		MBReleaseGroupID: p.ReleaseGroup.ID,
		MBRecordingID:    p.Track.RecordingID,
		MBTrackID:        p.Track.TrackID,

		// Recording info
		ISRC: p.Track.ISRC,

		// Artwork
		CoverArt: p.CoverArt,
	}
}

// RetagFile writes tags to a file in place without moving it.
// This is used by the retag feature to update existing library files.
func RetagFile(path string, data TagData) error {
	// Check file exists
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Detect file format from extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext != extMP3 && ext != extFLAC && ext != extOPUS && ext != extOGG && ext != extM4A && ext != extMP4 {
		return fmt.Errorf("unsupported file format: %s", ext)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()

	// Write tags with retry
	switch ext {
	case extMP3:
		return retryWithBackoff(ctx, "write MP3 tags", func() error {
			return writeMP3Tags(path, data)
		})
	case extFLAC:
		return retryWithBackoff(ctx, "write FLAC tags", func() error {
			return writeFLACTags(path, data)
		})
	case extOPUS, extOGG:
		return retryWithBackoff(ctx, "write Opus tags", func() error {
			return writeOpusTags(path, data)
		})
	case extM4A, extMP4:
		return retryWithBackoff(ctx, "write M4A tags", func() error {
			return writeM4ATags(path, data)
		})
	}

	return nil
}

// validateParams checks that all required parameters are present.
func validateParams(p ImportParams) error {
	if p.SourcePath == "" {
		return errors.New("source path is required")
	}
	if p.DestRoot == "" {
		return errors.New("destination root is required")
	}
	if p.ReleaseGroup == nil {
		return errors.New("release group is required")
	}
	if p.Release == nil {
		return errors.New("release is required")
	}
	if p.TrackIndex < 0 || p.TrackIndex >= len(p.Release.Tracks) {
		return fmt.Errorf("track index %d out of range (0-%d)", p.TrackIndex, len(p.Release.Tracks)-1)
	}
	return nil
}

// BuildGenreString builds a semicolon-separated genre string from multiple sources.
// Genres are title-cased (e.g., "rock" -> "Rock", "hard rock" -> "Hard Rock").
func BuildGenreString(releaseGenres, releaseGroupGenres []string) string {
	// Use release genres if available, otherwise fall back to release group genres
	genres := releaseGenres
	if len(genres) == 0 {
		genres = releaseGroupGenres
	}
	if len(genres) == 0 {
		return ""
	}

	// Title-case each genre and join with ";"
	titleCased := make([]string, len(genres))
	for i, g := range genres {
		titleCased[i] = TitleCase(g)
	}
	return strings.Join(titleCased, ";")
}

// TitleCase converts a string to title case (first letter of each word capitalized).
func TitleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if word != "" {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Close()
}

// moveFile moves a file from src to dst.
// Uses os.Rename if possible, otherwise copies and deletes.
func moveFile(src, dst string) error {
	// Try rename first (works if same filesystem)
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// Fall back to copy + delete
	if err := copyFile(src, dst); err != nil {
		return err
	}

	return os.Remove(src)
}

// retryWithBackoff executes an operation with exponential backoff retry.
// Returns the last error if all retries fail.
func retryWithBackoff(ctx context.Context, operation string, fn func() error) error {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return fmt.Errorf("%s: cancelled after %d attempts: %w", operation, attempt, lastErr)
			case <-time.After(backoff):
			}
			// Exponential backoff with cap
			backoff = min(backoff*2, maxBackoff)
		}

		// Execute with timeout
		done := make(chan error, 1)
		go func() {
			done <- fn()
		}()

		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: cancelled: %w", operation, ctx.Err())
		case err := <-done:
			if err == nil {
				return nil
			}
			lastErr = err
			// Check if error is retryable (file locks, temporary network issues)
			if !isRetryableError(err) {
				return fmt.Errorf("%s: %w", operation, err)
			}
			// Continue to retry
		case <-time.After(operationTimeout):
			lastErr = fmt.Errorf("timeout after %v", operationTimeout)
			// Continue to retry on timeout
		}
	}

	return fmt.Errorf("%s: failed after %d attempts: %w", operation, maxRetries+1, lastErr)
}

// isRetryableError checks if an error is likely temporary and worth retrying.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// File lock indicators
	if strings.Contains(errStr, "locked") ||
		strings.Contains(errStr, "in use") ||
		strings.Contains(errStr, "busy") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "access denied") {
		return true
	}

	// Network/IO indicators
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "i/o") ||
		strings.Contains(errStr, "temporary") {
		return true
	}

	// Check for OS-level errors that might be temporary
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	return false
}
