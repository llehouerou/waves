//nolint:goconst // test cases intentionally repeat strings for readability
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("Could not get home dir: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde expands to home",
			input:    "~/music",
			expected: filepath.Join(home, "music"),
		},
		{
			name:     "tilde with nested path",
			input:    "~/music/library/albums",
			expected: filepath.Join(home, "music", "library", "albums"),
		},
		{
			name:     "absolute path unchanged",
			input:    "/usr/local/music",
			expected: "/usr/local/music",
		},
		{
			name:     "relative path unchanged",
			input:    "music/albums",
			expected: "music/albums",
		},
		{
			name:     "empty string unchanged",
			input:    "",
			expected: "",
		},
		{
			name:     "tilde only",
			input:    "~",
			expected: home,
		},
		{
			name:     "tilde with slash",
			input:    "~/",
			expected: filepath.Join(home, ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetConfigPaths(t *testing.T) {
	paths := getConfigPaths()

	// Should have at least one path
	if len(paths) == 0 {
		t.Error("getConfigPaths() returned empty slice")
	}

	// Last path should be local config.toml
	lastPath := paths[len(paths)-1]
	if lastPath != "config.toml" {
		t.Errorf("last config path = %q, want %q", lastPath, "config.toml")
	}

	// If we have home dir, first path should be ~/.config/waves/config.toml
	if home, err := os.UserHomeDir(); err == nil {
		expectedFirst := filepath.Join(home, ".config", "waves", "config.toml")
		if paths[0] != expectedFirst {
			t.Errorf("first config path = %q, want %q", paths[0], expectedFirst)
		}
	}
}

func TestHasSlskdConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected bool
	}{
		{
			name: "both URL and APIKey set",
			config: Config{
				Slskd: SlskdConfig{
					URL:    "http://localhost:5030",
					APIKey: "my-api-key",
				},
			},
			expected: true,
		},
		{
			name: "only URL set",
			config: Config{
				Slskd: SlskdConfig{
					URL: "http://localhost:5030",
				},
			},
			expected: false,
		},
		{
			name: "only APIKey set",
			config: Config{
				Slskd: SlskdConfig{
					APIKey: "my-api-key",
				},
			},
			expected: false,
		},
		{
			name:     "neither set",
			config:   Config{},
			expected: false,
		},
		{
			name: "empty strings",
			config: Config{
				Slskd: SlskdConfig{
					URL:    "",
					APIKey: "",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.HasSlskdConfig()
			if result != tt.expected {
				t.Errorf("HasSlskdConfig() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHasLastfmConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected bool
	}{
		{
			name: "both APIKey and APISecret set",
			config: Config{
				Lastfm: LastfmConfig{
					APIKey:    "my-api-key",
					APISecret: "my-api-secret",
				},
			},
			expected: true,
		},
		{
			name: "only APIKey set",
			config: Config{
				Lastfm: LastfmConfig{
					APIKey: "my-api-key",
				},
			},
			expected: false,
		},
		{
			name: "only APISecret set",
			config: Config{
				Lastfm: LastfmConfig{
					APISecret: "my-api-secret",
				},
			},
			expected: false,
		},
		{
			name:     "neither set",
			config:   Config{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.HasLastfmConfig()
			if result != tt.expected {
				t.Errorf("HasLastfmConfig() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetRadioConfig_Defaults(t *testing.T) {
	// Empty config should get all defaults
	cfg := Config{}
	radio := cfg.GetRadioConfig()

	// Queue behavior
	if radio.BufferSize != 1 {
		t.Errorf("BufferSize = %d, want 1", radio.BufferSize)
	}

	// Artist selection
	if radio.SimilarArtistsLimit != 50 {
		t.Errorf("SimilarArtistsLimit = %d, want 50", radio.SimilarArtistsLimit)
	}
	if radio.ShufflePoolSize != 10 {
		t.Errorf("ShufflePoolSize = %d, want 10", radio.ShufflePoolSize)
	}
	if radio.ArtistsPerFill != 5 {
		t.Errorf("ArtistsPerFill = %d, want 5", radio.ArtistsPerFill)
	}
	if radio.ArtistMatchThreshold != 0.8 {
		t.Errorf("ArtistMatchThreshold = %f, want 0.8", radio.ArtistMatchThreshold)
	}

	// Variety enforcement
	if radio.MaxArtistRepeat != 2 {
		t.Errorf("MaxArtistRepeat = %d, want 2", radio.MaxArtistRepeat)
	}
	if radio.ArtistRepeatWindow != 20 {
		t.Errorf("ArtistRepeatWindow = %d, want 20", radio.ArtistRepeatWindow)
	}
	if radio.RecentSeedsWindow != 3 {
		t.Errorf("RecentSeedsWindow = %d, want 3", radio.RecentSeedsWindow)
	}

	// Scoring weights
	if radio.TopTrackBoost != 3.0 {
		t.Errorf("TopTrackBoost = %f, want 3.0", radio.TopTrackBoost)
	}
	if radio.UserBoost != 1.3 {
		t.Errorf("UserBoost = %f, want 1.3", radio.UserBoost)
	}
	if radio.FavoriteBoost != 2.0 {
		t.Errorf("FavoriteBoost = %f, want 2.0", radio.FavoriteBoost)
	}
	if radio.DecayFactor != 0.1 {
		t.Errorf("DecayFactor = %f, want 0.1", radio.DecayFactor)
	}
	if radio.MinSimilarityWeight != 0.1 {
		t.Errorf("MinSimilarityWeight = %f, want 0.1", radio.MinSimilarityWeight)
	}

	// Cache
	if radio.CacheTTLDays != 7 {
		t.Errorf("CacheTTLDays = %d, want 7", radio.CacheTTLDays)
	}
}

func TestGetRadioConfig_CustomValues(t *testing.T) {
	cfg := Config{
		Radio: RadioConfig{
			BufferSize:           5,
			SimilarArtistsLimit:  100,
			ShufflePoolSize:      20,
			ArtistsPerFill:       10,
			ArtistMatchThreshold: 0.9,
			MaxArtistRepeat:      3,
			ArtistRepeatWindow:   30,
			RecentSeedsWindow:    5,
			TopTrackBoost:        4.0,
			UserBoost:            1.5,
			FavoriteBoost:        2.5,
			DecayFactor:          0.2,
			MinSimilarityWeight:  0.2,
			CacheTTLDays:         14,
		},
	}

	radio := cfg.GetRadioConfig()

	if radio.BufferSize != 5 {
		t.Errorf("BufferSize = %d, want 5", radio.BufferSize)
	}
	if radio.SimilarArtistsLimit != 100 {
		t.Errorf("SimilarArtistsLimit = %d, want 100", radio.SimilarArtistsLimit)
	}
	if radio.ShufflePoolSize != 20 {
		t.Errorf("ShufflePoolSize = %d, want 20", radio.ShufflePoolSize)
	}
	if radio.ArtistsPerFill != 10 {
		t.Errorf("ArtistsPerFill = %d, want 10", radio.ArtistsPerFill)
	}
	if radio.ArtistMatchThreshold != 0.9 {
		t.Errorf("ArtistMatchThreshold = %f, want 0.9", radio.ArtistMatchThreshold)
	}
	if radio.MaxArtistRepeat != 3 {
		t.Errorf("MaxArtistRepeat = %d, want 3", radio.MaxArtistRepeat)
	}
	if radio.ArtistRepeatWindow != 30 {
		t.Errorf("ArtistRepeatWindow = %d, want 30", radio.ArtistRepeatWindow)
	}
	if radio.RecentSeedsWindow != 5 {
		t.Errorf("RecentSeedsWindow = %d, want 5", radio.RecentSeedsWindow)
	}
	if radio.TopTrackBoost != 4.0 {
		t.Errorf("TopTrackBoost = %f, want 4.0", radio.TopTrackBoost)
	}
	if radio.UserBoost != 1.5 {
		t.Errorf("UserBoost = %f, want 1.5", radio.UserBoost)
	}
	if radio.FavoriteBoost != 2.5 {
		t.Errorf("FavoriteBoost = %f, want 2.5", radio.FavoriteBoost)
	}
	if radio.DecayFactor != 0.2 {
		t.Errorf("DecayFactor = %f, want 0.2", radio.DecayFactor)
	}
	if radio.MinSimilarityWeight != 0.2 {
		t.Errorf("MinSimilarityWeight = %f, want 0.2", radio.MinSimilarityWeight)
	}
	if radio.CacheTTLDays != 14 {
		t.Errorf("CacheTTLDays = %d, want 14", radio.CacheTTLDays)
	}
}

func TestGetRadioConfig_InvalidValues(t *testing.T) {
	// Test that invalid values get replaced with defaults
	cfg := Config{
		Radio: RadioConfig{
			BufferSize:           25,   // > 20, should become 1
			SimilarArtistsLimit:  -1,   // negative, should become 50
			ArtistMatchThreshold: 1.5,  // > 1, should become 0.8
			DecayFactor:          -0.5, // negative, should become 0.1
			MinSimilarityWeight:  2.0,  // > 1, should become 0.1
		},
	}

	radio := cfg.GetRadioConfig()

	if radio.BufferSize != 1 {
		t.Errorf("BufferSize with invalid value = %d, want 1", radio.BufferSize)
	}
	if radio.SimilarArtistsLimit != 50 {
		t.Errorf("SimilarArtistsLimit with invalid value = %d, want 50", radio.SimilarArtistsLimit)
	}
	if radio.ArtistMatchThreshold != 0.8 {
		t.Errorf("ArtistMatchThreshold with invalid value = %f, want 0.8", radio.ArtistMatchThreshold)
	}
	if radio.DecayFactor != 0.1 {
		t.Errorf("DecayFactor with invalid value = %f, want 0.1", radio.DecayFactor)
	}
	if radio.MinSimilarityWeight != 0.1 {
		t.Errorf("MinSimilarityWeight with invalid value = %f, want 0.1", radio.MinSimilarityWeight)
	}
}

func TestGetRadioConfig_BoundaryValues(t *testing.T) {
	// Test boundary values
	tests := []struct {
		name                  string
		bufferSize            int
		expectedBufferSize    int
		threshold             float64
		expectedThreshold     float64
		decay                 float64
		expectedDecay         float64
		minSimilarity         float64
		expectedMinSimilarity float64
	}{
		{
			name:                  "buffer size at lower bound",
			bufferSize:            1,
			expectedBufferSize:    1,
			threshold:             0.1,
			expectedThreshold:     0.1,
			decay:                 0.01,
			expectedDecay:         0.01,
			minSimilarity:         0.01,
			expectedMinSimilarity: 0.01,
		},
		{
			name:                  "buffer size at upper bound",
			bufferSize:            20,
			expectedBufferSize:    20,
			threshold:             1.0,
			expectedThreshold:     1.0,
			decay:                 1.0,
			expectedDecay:         1.0,
			minSimilarity:         1.0,
			expectedMinSimilarity: 1.0,
		},
		{
			name:                  "buffer size zero becomes default",
			bufferSize:            0,
			expectedBufferSize:    1,
			threshold:             0.0,
			expectedThreshold:     0.8,
			decay:                 0.0,
			expectedDecay:         0.1,
			minSimilarity:         0.0,
			expectedMinSimilarity: 0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Radio: RadioConfig{
					BufferSize:           tt.bufferSize,
					ArtistMatchThreshold: tt.threshold,
					DecayFactor:          tt.decay,
					MinSimilarityWeight:  tt.minSimilarity,
				},
			}
			radio := cfg.GetRadioConfig()

			if radio.BufferSize != tt.expectedBufferSize {
				t.Errorf("BufferSize = %d, want %d", radio.BufferSize, tt.expectedBufferSize)
			}
			if radio.ArtistMatchThreshold != tt.expectedThreshold {
				t.Errorf("ArtistMatchThreshold = %f, want %f", radio.ArtistMatchThreshold, tt.expectedThreshold)
			}
			if radio.DecayFactor != tt.expectedDecay {
				t.Errorf("DecayFactor = %f, want %f", radio.DecayFactor, tt.expectedDecay)
			}
			if radio.MinSimilarityWeight != tt.expectedMinSimilarity {
				t.Errorf("MinSimilarityWeight = %f, want %f", radio.MinSimilarityWeight, tt.expectedMinSimilarity)
			}
		})
	}
}

func TestLoad_EmptyConfig(t *testing.T) {
	// Create temp directory with empty config
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("could not change to temp directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	// Create an empty config file
	if err := os.WriteFile("config.toml", []byte(""), 0o600); err != nil {
		t.Fatalf("could not write config file: %v", err)
	}

	// Load should succeed even with empty config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Note: Values may be inherited from ~/.config/waves/config.toml if it exists
	// We just verify Load() succeeds and returns a valid config
}

func TestLoad_BasicConfig(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("could not change to temp directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	// Create config file
	configContent := `
icons = "nerd"
library_sources = ["/music", "~/library"]

[slskd]
url = "http://localhost:5030/"
apikey = "test-key"
`
	if err := os.WriteFile("config.toml", []byte(configContent), 0o600); err != nil {
		t.Fatalf("could not write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check values
	if cfg.Icons != "nerd" {
		t.Errorf("Icons = %q, want %q", cfg.Icons, "nerd")
	}

	// Check that URL trailing slash is removed
	if cfg.Slskd.URL != "http://localhost:5030" {
		t.Errorf("Slskd.URL = %q, want %q", cfg.Slskd.URL, "http://localhost:5030")
	}

	if cfg.Slskd.APIKey != "test-key" {
		t.Errorf("Slskd.APIKey = %q, want %q", cfg.Slskd.APIKey, "test-key")
	}

	// Check library sources - first should be absolute, second should be expanded
	if len(cfg.LibrarySources) != 2 {
		t.Fatalf("LibrarySources length = %d, want 2", len(cfg.LibrarySources))
	}

	if cfg.LibrarySources[0] != "/music" {
		t.Errorf("LibrarySources[0] = %q, want %q", cfg.LibrarySources[0], "/music")
	}

	// Second source should have ~ expanded
	home, _ := os.UserHomeDir()
	expectedSecond := filepath.Join(home, "library")
	if cfg.LibrarySources[1] != expectedSecond {
		t.Errorf("LibrarySources[1] = %q, want %q", cfg.LibrarySources[1], expectedSecond)
	}
}

func TestLoad_InvalidToml(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("could not change to temp directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	// Create invalid config file
	if err := os.WriteFile("config.toml", []byte("invalid = [[["), 0o600); err != nil {
		t.Fatalf("could not write config file: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Error("Load() expected error for invalid TOML, got nil")
	}
}

func TestLoad_DefaultFolderExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("could not change to temp directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	configContent := `default_folder = "~/music"`
	if err := os.WriteFile("config.toml", []byte(configContent), 0o600); err != nil {
		t.Fatalf("could not write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "music")
	if cfg.DefaultFolder != expected {
		t.Errorf("DefaultFolder = %q, want %q", cfg.DefaultFolder, expected)
	}
}

func TestLoad_SlskdCompletedPathExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not get working directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("could not change to temp directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	configContent := `
[slskd]
completed_path = "~/downloads/complete"
`
	if err := os.WriteFile("config.toml", []byte(configContent), 0o600); err != nil {
		t.Fatalf("could not write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "downloads", "complete")
	if cfg.Slskd.CompletedPath != expected {
		t.Errorf("Slskd.CompletedPath = %q, want %q", cfg.Slskd.CompletedPath, expected)
	}
}
