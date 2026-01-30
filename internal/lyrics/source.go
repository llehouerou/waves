package lyrics

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/llehouerou/waves/internal/lrclib"
)

// Source provides lyrics from local files, cache, or the lrclib API.
type Source struct {
	client   *lrclib.Client
	cacheDir string
}

// NewSource creates a new lyrics source.
func NewSource() *Source {
	cacheDir := getCacheDir()
	return &Source{
		client:   lrclib.New(),
		cacheDir: cacheDir,
	}
}

// getCacheDir returns the lyrics cache directory.
func getCacheDir() string {
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return filepath.Join(cacheHome, "waves", "lyrics")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "waves", "lyrics")
}

// TrackInfo contains the information needed to fetch lyrics.
type TrackInfo struct {
	FilePath string // Path to audio file (for local .lrc lookup)
	Artist   string
	Title    string
	Album    string
	Duration time.Duration
}

// FetchResult contains the result of a lyrics fetch.
type FetchResult struct {
	Lyrics *Lyrics
	Source string // "local", "cache", "api", or "not_found"
	Err    error
}

// Fetch retrieves lyrics for a track using the priority order:
// 1. Local .lrc file (same directory as audio file)
// 2. Cached .lrc file
// 3. lrclib API (and cache the result)
func (s *Source) Fetch(ctx context.Context, track TrackInfo) FetchResult {
	// 1. Try local file
	if track.FilePath != "" {
		localPath := lrcPathForAudio(track.FilePath)
		if lyrics, err := s.loadFromFile(localPath); err == nil && lyrics != nil {
			return FetchResult{Lyrics: lyrics, Source: "local"}
		}
	}

	// Need artist and title for cache/API lookup
	if track.Artist == "" || track.Title == "" {
		return FetchResult{Source: "not_found"}
	}

	// 2. Try cache
	cachePath := s.cachePath(track.Artist, track.Title)
	if lyrics, err := s.loadFromFile(cachePath); err == nil && lyrics != nil {
		return FetchResult{Lyrics: lyrics, Source: "cache"}
	}

	// 3. Try API
	return s.fetchFromAPI(ctx, track)
}

// fetchFromAPI fetches lyrics from the lrclib API.
func (s *Source) fetchFromAPI(ctx context.Context, track TrackInfo) FetchResult {
	result, err := s.client.Get(ctx, track.Artist, track.Title, track.Duration)
	if err != nil {
		// ErrNotFound is not a real error, just means no lyrics available
		if errors.Is(err, lrclib.ErrNotFound) {
			return FetchResult{Source: "not_found"}
		}
		return FetchResult{Source: "not_found", Err: err}
	}

	lyrics := s.parseLyricsResult(result)
	if lyrics == nil || len(lyrics.Lines) == 0 {
		return FetchResult{Source: "not_found"}
	}

	// Cache the result
	if result.HasSyncedLyrics() {
		_ = s.saveToCache(track.Artist, track.Title, result.SyncedLyrics)
	}

	return FetchResult{Lyrics: lyrics, Source: "api"}
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
