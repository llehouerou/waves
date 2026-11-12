package lyrics

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/llehouerou/waves/internal/lrclib"
)

// ErrNotFound is returned by Fetch for a track nobody has lyrics for.
var ErrNotFound = lrclib.ErrNotFound

// Client is the part of the lrclib client a source asks.
type Client interface {
	Get(ctx context.Context, artist, title string, duration time.Duration) (*lrclib.LyricsResult, error)
}

// Source provides lyrics from local files, cache, or the lrclib API.
type Source struct {
	client   Client
	cacheDir string
}

// NewSource creates a lyrics source that caches what client finds in
// cacheDir; an empty cacheDir is no cache.
func NewSource(cacheDir string, client Client) *Source {
	return &Source{client: client, cacheDir: cacheDir}
}

// TrackInfo contains the information needed to fetch lyrics.
type TrackInfo struct {
	FilePath string // Path to audio file (for local .lrc lookup)
	Artist   string
	Title    string
	Album    string
	Duration time.Duration
}

// Fetch retrieves lyrics for a track using the priority order:
// 1. Local .lrc file (same directory as audio file)
// 2. Cached .lrc file
// 3. lrclib API (and cache the result)
//
// It returns ErrNotFound when none has them.
func (s *Source) Fetch(ctx context.Context, track TrackInfo) (*Lyrics, error) {
	// 1. Try local file
	if track.FilePath != "" {
		localPath := lrcPathForAudio(track.FilePath)
		if lyrics, err := s.loadFromFile(localPath); err == nil && lyrics != nil {
			return lyrics, nil
		}
	}

	// Need artist and title for cache/API lookup
	if track.Artist == "" || track.Title == "" {
		return nil, ErrNotFound
	}

	// 2. Try cache
	cachePath := s.cachePath(track.Artist, track.Title)
	if lyrics, err := s.loadFromFile(cachePath); err == nil && lyrics != nil {
		return lyrics, nil
	}

	// 3. Try API
	return s.fetchFromAPI(ctx, track)
}

// fetchFromAPI fetches lyrics from the lrclib API.
func (s *Source) fetchFromAPI(ctx context.Context, track TrackInfo) (*Lyrics, error) {
	result, err := s.client.Get(ctx, track.Artist, track.Title, track.Duration)
	if err != nil {
		return nil, err // lrclib's own ErrNotFound when it has none
	}

	lyrics := s.parseLyricsResult(result)
	if lyrics == nil || len(lyrics.Lines) == 0 {
		return nil, ErrNotFound
	}

	// Cache the result
	if result.HasSyncedLyrics() {
		_ = s.saveToCache(track.Artist, track.Title, result.SyncedLyrics)
	}

	return lyrics, nil
}

// parseLyricsResult parses the API result into a Lyrics struct.
func (s *Source) parseLyricsResult(result *lrclib.LyricsResult) *Lyrics {
	var lyrics *Lyrics
	if result.HasSyncedLyrics() {
		var err error
		lyrics, err = ParseLRC(strings.NewReader(result.SyncedLyrics))
		if err != nil {
			return nil
		}
	} else if result.HasPlainLyrics() {
		// Create unsynced lyrics (all at time 0)
		lyrics = &Lyrics{
			Title:  result.TrackName,
			Artist: result.ArtistName,
			Album:  result.AlbumName,
		}
		for line := range strings.SplitSeq(result.PlainLyrics, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				lyrics.Lines = append(lyrics.Lines, Line{Time: 0, Text: line})
			}
		}
	}

	if lyrics == nil {
		return nil
	}

	// Fill in metadata if missing
	if lyrics.Artist == "" {
		lyrics.Artist = result.ArtistName
	}
	if lyrics.Title == "" {
		lyrics.Title = result.TrackName
	}
	if lyrics.Album == "" {
		lyrics.Album = result.AlbumName
	}

	return lyrics
}

// lrcPathForAudio returns the expected .lrc file path for an audio file.
func lrcPathForAudio(audioPath string) string {
	ext := filepath.Ext(audioPath)
	return audioPath[:len(audioPath)-len(ext)] + ".lrc"
}

// loadFromFile loads lyrics from an LRC file.
func (s *Source) loadFromFile(path string) (*Lyrics, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseLRC(f)
}

// cachePath returns the cache file path for a track.
func (s *Source) cachePath(artist, title string) string {
	if s.cacheDir == "" {
		return ""
	}
	return filepath.Join(s.cacheDir, sanitizeFilename(artist), sanitizeFilename(title)+".lrc")
}

// saveToCache saves LRC content to the cache directory.
func (s *Source) saveToCache(artist, title, content string) error {
	if s.cacheDir == "" {
		return nil
	}

	path := s.cachePath(artist, title)
	if path == "" {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(content), 0o600)
}

// sanitizeFilename removes or replaces characters that are problematic in filenames.
var invalidFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

func sanitizeFilename(name string) string {
	// Replace invalid characters with underscore
	name = invalidFilenameChars.ReplaceAllString(name, "_")
	// Trim spaces and dots from ends
	name = strings.Trim(name, " .")
	// Limit length
	if len(name) > 100 {
		name = name[:100]
	}
	if name == "" {
		name = "_"
	}
	return name
}

// IsSynced returns true if the lyrics have timestamps (synced).
func (l *Lyrics) IsSynced() bool {
	if len(l.Lines) == 0 {
		return false
	}
	// Check if any line has a non-zero timestamp
	for _, line := range l.Lines {
		if line.Time > 0 {
			return true
		}
	}
	return false
}
