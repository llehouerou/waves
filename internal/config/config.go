package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/llehouerou/waves/internal/rename"
)

type Config struct {
	DefaultFolder  string   `koanf:"default_folder"`
	Icons          string   `koanf:"icons"`           // "nerd", "unicode", or "none"
	LibrarySources []string `koanf:"library_sources"` // paths to scan for music library

	// slskd integration (enables download popup via gd keybinding when configured)
	Slskd SlskdConfig `koanf:"slskd"`

	// MusicBrainz settings
	MusicBrainz MusicBrainzConfig `koanf:"musicbrainz"`

	// Last.fm scrobbling (enables scrobbling when configured)
	Lastfm LastfmConfig `koanf:"lastfm"`

	// Radio mode settings
	Radio RadioConfig `koanf:"radio"`

	// Rename pattern settings
	Rename RenameConfig `koanf:"rename"`

	// Desktop notifications
	Notifications NotificationsConfig `koanf:"notifications"`
}

// SlskdConfig holds all slskd-related configuration.
type SlskdConfig struct {
	URL           string       `koanf:"url"`            // e.g., "http://localhost:5030"
	APIKey        string       `koanf:"apikey"`         // API key from slskd settings
	CompletedPath string       `koanf:"completed_path"` // Path to completed downloads folder
	Filters       SlskdFilters `koanf:"filters"`        // default search filters
}

// SlskdFilters defines default filters for slskd search results.
type SlskdFilters struct {
	Format     string `koanf:"format"`      // "both", "lossless", "lossy" (default: "both")
	NoSlot     *bool  `koanf:"no_slot"`     // filter users with no free slot (default: true)
	TrackCount *bool  `koanf:"track_count"` // filter by track count (default: true)
}

// MusicBrainzConfig holds MusicBrainz-related configuration.
type MusicBrainzConfig struct {
	AlbumsOnly *bool `koanf:"albums_only"` // filter release groups to albums only (default: true)
}

// LastfmConfig holds Last.fm scrobbling configuration.
type LastfmConfig struct {
	APIKey    string `koanf:"api_key"`
	APISecret string `koanf:"api_secret"`
}

// RadioConfig holds Last.fm radio mode configuration.
type RadioConfig struct {
	// Queue behavior
	BufferSize int `koanf:"buffer_size"` // Tracks to queue ahead (1-20, default: 1)

	// Artist selection
	SimilarArtistsLimit  int     `koanf:"similar_artists_limit"`  // Similar artists to fetch from API (default: 50)
	ShufflePoolSize      int     `koanf:"shuffle_pool_size"`      // Top N artists to shuffle from (default: 10)
	ArtistsPerFill       int     `koanf:"artists_per_fill"`       // Artists to use per fill after shuffle (default: 5)
	ArtistMatchThreshold float64 `koanf:"artist_match_threshold"` // Fuzzy match threshold 0.0-1.0 (default: 0.8)

	// Variety enforcement
	MaxArtistRepeat    int `koanf:"max_artist_repeat"`    // Max times same artist in window (default: 2)
	ArtistRepeatWindow int `koanf:"artist_repeat_window"` // Window size for repeat check (default: 20)
	RecentSeedsWindow  int `koanf:"recent_seeds_window"`  // Seeds to remember for A→B→A prevention (default: 3)

	// Scoring weights
	TopTrackBoost       float64 `koanf:"top_track_boost"`       // Boost multiplier for top tracks (default: 3.0)
	UserBoost           float64 `koanf:"user_boost"`            // Multiplier for user-scrobbled tracks (default: 1.3)
	FavoriteBoost       float64 `koanf:"favorite_boost"`        // Multiplier for favorite tracks, replaces user_boost (default: 2.0)
	DecayFactor         float64 `koanf:"decay_factor"`          // Penalty for recently played (default: 0.1)
	MinSimilarityWeight float64 `koanf:"min_similarity_weight"` // Floor for similarity score (default: 0.1)

	// Cache
	CacheTTLDays int `koanf:"cache_ttl_days"` // Cache TTL in days (default: 7)

	// Unused
	ExplorationDepth int `koanf:"exploration_depth"` // Reserved for future use
}

// RenameConfig holds file renaming configuration.
type RenameConfig struct {
	Folder   string `koanf:"folder"`   // Template for folder path
	Filename string `koanf:"filename"` // Template for filename (without extension)

	// Smart features (nil means use default=true)
	ReissueNotation   *bool `koanf:"reissue_notation"`   // [YYYY reissue] suffix
	VABrackets        *bool `koanf:"va_brackets"`        // [Various Artists] folder
	SinglesHandling   *bool `koanf:"singles_handling"`   // [singles] folder, no album in filename
	ReleaseTypeNotes  *bool `koanf:"release_type_notes"` // (soundtrack), (live), etc.
	AndToAmpersand    *bool `koanf:"and_to_ampersand"`   // "and" → "&"
	RemoveFeat        *bool `koanf:"remove_feat"`        // Strip "feat." from titles
	EllipsisNormalize *bool `koanf:"ellipsis_normalize"` // "..." → "…"
}

// NotificationsConfig holds desktop notification settings.
type NotificationsConfig struct {
	Enabled      *bool `koanf:"enabled"`        // Master toggle (default: true)
	NowPlaying   *bool `koanf:"now_playing"`    // On track change (default: true)
	Downloads    *bool `koanf:"downloads"`      // On download complete (default: true)
	Errors       *bool `koanf:"errors"`         // On errors (default: true)
	ShowAlbumArt *bool `koanf:"show_album_art"` // Include album art (default: true)
	Timeout      int32 `koanf:"timeout"`        // ms, 0 = don't expire (default: 5000)
}

// ToRenameConfig converts the config RenameConfig to a rename.Config,
// applying defaults for nil values.
func (c RenameConfig) ToRenameConfig() rename.Config {
	cfg := rename.DefaultConfig()

	// Override templates if specified
	if c.Folder != "" {
		cfg.Folder = c.Folder
	}
	if c.Filename != "" {
		cfg.Filename = c.Filename
	}

	// Override toggles if explicitly set
	if c.ReissueNotation != nil {
		cfg.ReissueNotation = *c.ReissueNotation
	}
	if c.VABrackets != nil {
		cfg.VABrackets = *c.VABrackets
	}
	if c.SinglesHandling != nil {
		cfg.SinglesHandling = *c.SinglesHandling
	}
	if c.ReleaseTypeNotes != nil {
		cfg.ReleaseTypeNotes = *c.ReleaseTypeNotes
	}
	if c.AndToAmpersand != nil {
		cfg.AndToAmpersand = *c.AndToAmpersand
	}
	if c.RemoveFeat != nil {
		cfg.RemoveFeat = *c.RemoveFeat
	}
	if c.EllipsisNormalize != nil {
		cfg.EllipsisNormalize = *c.EllipsisNormalize
	}

	return cfg
}

func Load() (*Config, error) {
	k := koanf.New(".")

	// Try config files in order of priority (last wins)
	configPaths := getConfigPaths()

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			if err := k.Load(file.Provider(path), toml.Parser()); err != nil {
				return nil, err
			}
		}
	}

	cfg := &Config{
		DefaultFolder: "", // empty means use cwd
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, err
	}

	// Expand ~ in default_folder
	if cfg.DefaultFolder != "" {
		cfg.DefaultFolder = expandPath(cfg.DefaultFolder)
	}

	// Expand ~ in library_sources
	for i, src := range cfg.LibrarySources {
		cfg.LibrarySources[i] = expandPath(src)
	}

	// Normalize slskd URL (remove trailing slash)
	cfg.Slskd.URL = strings.TrimSuffix(cfg.Slskd.URL, "/")

	// Expand ~ in slskd completed_path
	if cfg.Slskd.CompletedPath != "" {
		cfg.Slskd.CompletedPath = expandPath(cfg.Slskd.CompletedPath)
	}

	return cfg, nil
}

func getConfigPaths() []string {
	paths := []string{}

	// 1. ~/.config/waves/config.toml
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "waves", "config.toml"))
	}

	// 2. ./config.toml (pwd, highest priority)
	paths = append(paths, "config.toml")

	return paths
}

func expandPath(path string) string {
	if path != "" && path[0] == '~' {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// HasSlskdConfig returns true if slskd integration is configured.
func (c *Config) HasSlskdConfig() bool {
	return c.Slskd.URL != "" && c.Slskd.APIKey != ""
}

// HasLastfmConfig returns true if Last.fm scrobbling is configured.
func (c *Config) HasLastfmConfig() bool {
	return c.Lastfm.APIKey != "" && c.Lastfm.APISecret != ""
}

// GetRadioConfig returns the radio configuration with defaults applied.
func (c *Config) GetRadioConfig() RadioConfig {
	cfg := c.Radio

	// Queue behavior
	if cfg.BufferSize <= 0 || cfg.BufferSize > 20 {
		cfg.BufferSize = 1
	}

	// Artist selection
	if cfg.SimilarArtistsLimit <= 0 {
		cfg.SimilarArtistsLimit = 50
	}
	if cfg.ShufflePoolSize <= 0 {
		cfg.ShufflePoolSize = 10
	}
	if cfg.ArtistsPerFill <= 0 {
		cfg.ArtistsPerFill = 5
	}
	if cfg.ArtistMatchThreshold <= 0 || cfg.ArtistMatchThreshold > 1 {
		cfg.ArtistMatchThreshold = 0.8
	}

	// Variety enforcement
	if cfg.MaxArtistRepeat <= 0 {
		cfg.MaxArtistRepeat = 2
	}
	if cfg.ArtistRepeatWindow <= 0 {
		cfg.ArtistRepeatWindow = 20
	}
	if cfg.RecentSeedsWindow <= 0 {
		cfg.RecentSeedsWindow = 3
	}

	// Scoring weights
	if cfg.TopTrackBoost <= 0 {
		cfg.TopTrackBoost = 3.0
	}
	if cfg.UserBoost <= 0 {
		cfg.UserBoost = 1.3
	}
	if cfg.FavoriteBoost <= 0 {
		cfg.FavoriteBoost = 2.0
	}
	if cfg.DecayFactor <= 0 || cfg.DecayFactor > 1 {
		cfg.DecayFactor = 0.1
	}
	if cfg.MinSimilarityWeight <= 0 || cfg.MinSimilarityWeight > 1 {
		cfg.MinSimilarityWeight = 0.1
	}

	// Cache
	if cfg.CacheTTLDays <= 0 {
		cfg.CacheTTLDays = 7
	}

	return cfg
}

// GetNotificationsConfig returns the notification configuration with defaults applied.
func (c *Config) GetNotificationsConfig() NotificationsConfig {
	cfg := c.Notifications

	// Apply defaults for nil pointers
	// Notifications are opt-in (disabled by default)
	if cfg.Enabled == nil {
		f := false
		cfg.Enabled = &f
	}
	if cfg.NowPlaying == nil {
		t := true
		cfg.NowPlaying = &t
	}
	if cfg.Downloads == nil {
		t := true
		cfg.Downloads = &t
	}
	if cfg.Errors == nil {
		t := true
		cfg.Errors = &t
	}
	if cfg.ShowAlbumArt == nil {
		t := true
		cfg.ShowAlbumArt = &t
	}

	// Apply default timeout
	if cfg.Timeout == 0 {
		cfg.Timeout = 5000
	}

	return cfg
}
