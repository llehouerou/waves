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

	// slskd integration (enables download popup via gd keybinding when both are set)
	SlskdURL    string `koanf:"slskd_url"`    // e.g., "http://localhost:5030"
	SlskdAPIKey string `koanf:"slskd_apikey"` // API key from slskd settings
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
	cfg.SlskdURL = strings.TrimSuffix(cfg.SlskdURL, "/")

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
	return c.SlskdURL != "" && c.SlskdAPIKey != ""
}
