package config

import (
	"os"
	"path/filepath"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	DefaultFolder string `koanf:"default_folder"`
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
