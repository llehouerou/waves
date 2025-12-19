package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
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
	BufferSize           int     `koanf:"buffer_size"`            // Number of tracks to queue ahead (1-20, default: 1)
	ExplorationDepth     int     `koanf:"exploration_depth"`      // Depth for related artists (1 = direct only, default: 1)
	CacheTTLDays         int     `koanf:"cache_ttl_days"`         // Cache TTL in days (default: 7)
	ArtistMatchThreshold float64 `koanf:"artist_match_threshold"` // Fuzzy match threshold (0.0-1.0, default: 0.8)
	UserBoost            float64 `koanf:"user_boost"`             // Multiplier for scrobbled tracks (default: 1.3)
	DecayFactor          float64 `koanf:"decay_factor"`           // Score multiplier for recently played (default: 0.1)
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

	// Apply defaults
	if cfg.BufferSize <= 0 || cfg.BufferSize > 20 {
		cfg.BufferSize = 1
	}
	if cfg.ExplorationDepth <= 0 {
		cfg.ExplorationDepth = 1
	}
	if cfg.CacheTTLDays <= 0 {
		cfg.CacheTTLDays = 7
	}
	if cfg.ArtistMatchThreshold <= 0 || cfg.ArtistMatchThreshold > 1 {
		cfg.ArtistMatchThreshold = 0.8
	}
	if cfg.UserBoost <= 0 {
		cfg.UserBoost = 1.3
	}
	if cfg.DecayFactor <= 0 || cfg.DecayFactor > 1 {
		cfg.DecayFactor = 0.1
	}

	return cfg
}
