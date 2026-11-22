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
// 2. Cached lyrics: a .lrc file for synced ones, a .txt file for unsynced ones,
// kept for the same artist, title and duration as lrclib is asked with
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
	if lyrics := s.loadFromCache(track); lyrics != nil {
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

	lyrics, ext, content := parseLyricsResult(result)
	if lyrics == nil {
		return nil, ErrNotFound
	}

	_ = s.saveToCache(s.cachePath(track, ext), content) //nolint:errcheck // a cache that cannot be written only costs a fetch next time

	return lyrics, nil
}

// parseLyricsResult parses the API result into a Lyrics struct, with the
// extension and the content to cache it as; nil when it has no line. Synced
// lyrics that give no timed line are no synced lyrics: the plain ones of the
// result are taken instead.
func parseLyricsResult(result *lrclib.LyricsResult) (lyrics *Lyrics, ext, content string) {
	if result.HasSyncedLyrics() {
		synced, err := ParseLRC(strings.NewReader(result.SyncedLyrics))
		if err == nil && len(synced.Lines) > 0 {
			lyrics, ext, content = synced, syncedExt, result.SyncedLyrics
		}
	}
	if lyrics == nil {
		lines := plainLines(result.PlainLyrics)
		if len(lines) == 0 {
			return nil, "", ""
		}
		// Unsynced lyrics: every line at time 0.
		lyrics, ext, content = &Lyrics{Lines: lines}, plainExt, result.PlainLyrics
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

	return lyrics, ext, content
}

// plainLines are the lines of plain lyrics: each non-blank line, trimmed, as
// text at time 0, so the lyrics are unsynced.
func plainLines(text string) []Line {
	var lines []Line
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, Line{Time: 0, Text: line})
		}
	}
	return lines
}

// lrcPathForAudio returns the expected .lrc file path for an audio file.
func lrcPathForAudio(audioPath string) string {
	ext := filepath.Ext(audioPath)
	return audioPath[:len(audioPath)-len(ext)] + ".lrc"
}

// loadFromFile loads lyrics from an LRC file. A file with no timed line, empty
// or plain text, has no lyrics: ErrNotFound.
func (s *Source) loadFromFile(path string) (*Lyrics, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	lyrics, err := ParseLRC(f)
	if err != nil {
		return nil, err
	}
	if len(lyrics.Lines) == 0 {
		return nil, ErrNotFound
	}
	return lyrics, nil
}

// loadFromCache returns the lyrics cached for a track, synced or unsynced,
// or nil when none are or there is no cache.
func (s *Source) loadFromCache(track TrackInfo) *Lyrics {
	if lyrics, err := s.loadFromFile(s.cachePath(track, syncedExt)); err == nil && lyrics != nil {
		return lyrics
	}
	// Plain text, not LRC: ParseLRC would drop its lines or read a
	// bracketed time in the text as a timestamp.
	text, err := os.ReadFile(s.cachePath(track, plainExt))
	if err != nil {
		return nil
	}
	lines := plainLines(string(text))
	if len(lines) == 0 {
		return nil
	}
	return &Lyrics{Lines: lines}
}

// The extensions of the cached lyrics of a track: synced ones as LRC,
// unsynced ones as the plain text lrclib answered with.
const (
	syncedExt = ".lrc"
	plainExt  = ".txt"
)

// cachePath returns the cache file path of the lyrics of a track with ext,
// syncedExt or plainExt; "" without a cache. The lyrics are lrclib's answer
// for the track's artist, title and duration, so the path carries the three:
// a folder per artist, one per title in it, and a file per duration as lrclib
// is asked with it (noDuration when it is asked with none), which the length
// limit on the names of the folders does not cut.
func (s *Source) cachePath(track TrackInfo, ext string) string {
	if s.cacheDir == "" {
		return ""
	}
	name := noDuration
	if seconds, ok := lrclib.Seconds(track.Duration); ok {
		name = seconds + "s"
	}
	return filepath.Join(s.cacheDir, sanitizeFilename(track.Artist), sanitizeFilename(track.Title), name+ext)
}

// noDuration is the name of the cached lyrics of a track lrclib is asked for
// with no duration.
const noDuration = "no-duration"

// saveToCache writes content to path in the cache directory; an empty path
// is no cache.
func (s *Source) saveToCache(path, content string) error {
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
