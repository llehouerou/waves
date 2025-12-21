//nolint:goconst // test cases intentionally repeat strings for readability
package icons

import (
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name          string
		style         string
		expectedStyle Style
	}{
		{"nerd style", "nerd", StyleNerd},
		{"unicode style", "unicode", StyleUnicode},
		{"none style", "none", StyleNone},
		{"empty string defaults to none", "", StyleNone},
		{"unknown style defaults to none", "invalid", StyleNone},
		{"case sensitive - NERD defaults to none", "NERD", StyleNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.style)

			// Verify by checking a known icon
			switch tt.expectedStyle {
			case StyleNerd:
				if current != nerdIcons {
					t.Error("expected nerd icons to be active")
				}
			case StyleUnicode:
				if current != unicodeIcons {
					t.Error("expected unicode icons to be active")
				}
			case StyleNone:
				if current != noneIcons {
					t.Error("expected none icons to be active")
				}
			}
		})
	}

	// Reset to default
	Init("none")
}

func TestFolder(t *testing.T) {
	tests := []struct {
		style    string
		expected string
	}{
		{"none", "/"},
		{"nerd", "\uf07b "},
		{"unicode", "📁 "},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			Init(tt.style)
			if got := Folder(); got != tt.expected {
				t.Errorf("Folder() = %q, want %q", got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestIsPrefix(t *testing.T) {
	tests := []struct {
		style    string
		expected bool
	}{
		{"none", false},
		{"nerd", true},
		{"unicode", true},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			Init(tt.style)
			if got := IsPrefix(); got != tt.expected {
				t.Errorf("IsPrefix() = %v, want %v", got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestFormatDir(t *testing.T) {
	tests := []struct {
		style    string
		name     string
		expected string
	}{
		{"none", "Music", "Music/"},
		{"nerd", "Music", "\uf07b Music"},
		{"unicode", "Music", "📁 Music"},
		{"none", "", "/"},
		{"nerd", "", "\uf07b "},
	}

	for _, tt := range tests {
		t.Run(tt.style+"_"+tt.name, func(t *testing.T) {
			Init(tt.style)
			if got := FormatDir(tt.name); got != tt.expected {
				t.Errorf("FormatDir(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestFormatAudio(t *testing.T) {
	tests := []struct {
		style    string
		name     string
		expected string
	}{
		{"none", "song.mp3", "song.mp3"},
		{"nerd", "song.mp3", "\uf001 song.mp3"},
		{"unicode", "song.mp3", "🎵 song.mp3"},
		{"none", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.style+"_"+tt.name, func(t *testing.T) {
			Init(tt.style)
			if got := FormatAudio(tt.name); got != tt.expected {
				t.Errorf("FormatAudio(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestFormatArtist(t *testing.T) {
	tests := []struct {
		style    string
		name     string
		expected string
	}{
		{"none", "Artist Name", "Artist Name"},
		{"nerd", "Artist Name", "\uf007 Artist Name"},
		{"unicode", "Artist Name", "👤 Artist Name"},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			Init(tt.style)
			if got := FormatArtist(tt.name); got != tt.expected {
				t.Errorf("FormatArtist(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestFormatAlbum(t *testing.T) {
	tests := []struct {
		style    string
		name     string
		expected string
	}{
		{"none", "Album Title", "Album Title"},
		{"nerd", "Album Title", "󰀥 Album Title"},
		{"unicode", "Album Title", "💿 Album Title"},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			Init(tt.style)
			if got := FormatAlbum(tt.name); got != tt.expected {
				t.Errorf("FormatAlbum(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestFormatPlaylist(t *testing.T) {
	tests := []struct {
		style    string
		name     string
		expected string
	}{
		{"none", "My Playlist", "My Playlist"},
		{"nerd", "My Playlist", "󰲸 My Playlist"},
		{"unicode", "My Playlist", "📋 My Playlist"},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			Init(tt.style)
			if got := FormatPlaylist(tt.name); got != tt.expected {
				t.Errorf("FormatPlaylist(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestShuffle(t *testing.T) {
	tests := []struct {
		style    string
		expected string
	}{
		{"none", "[S]"},
		{"nerd", "󰒟"},
		{"unicode", "🔀"},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			Init(tt.style)
			if got := Shuffle(); got != tt.expected {
				t.Errorf("Shuffle() = %q, want %q", got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestRepeatAll(t *testing.T) {
	tests := []struct {
		style    string
		expected string
	}{
		{"none", "[R]"},
		{"nerd", "󰑖"},
		{"unicode", "🔁"},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			Init(tt.style)
			if got := RepeatAll(); got != tt.expected {
				t.Errorf("RepeatAll() = %q, want %q", got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestRepeatOne(t *testing.T) {
	tests := []struct {
		style    string
		expected string
	}{
		{"none", "[1]"},
		{"nerd", "󰑘"},
		{"unicode", "🔂"},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			Init(tt.style)
			if got := RepeatOne(); got != tt.expected {
				t.Errorf("RepeatOne() = %q, want %q", got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestFavorite(t *testing.T) {
	tests := []struct {
		style    string
		expected string
	}{
		{"none", "*"},
		{"nerd", "󰣐"},
		{"unicode", "♥"},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			Init(tt.style)
			if got := Favorite(); got != tt.expected {
				t.Errorf("Favorite() = %q, want %q", got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestRadio(t *testing.T) {
	tests := []struct {
		style    string
		expected string
	}{
		{"none", "[~]"},
		{"nerd", "󰐹"},
		{"unicode", "📻"},
	}

	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			Init(tt.style)
			if got := Radio(); got != tt.expected {
				t.Errorf("Radio() = %q, want %q", got, tt.expected)
			}
		})
	}

	Init("none")
}

func TestIconsStructure(t *testing.T) {
	// Verify that all icon sets have non-empty values for key icons
	sets := []struct {
		name  string
		icons Icons
	}{
		{"nerd", nerdIcons},
		{"unicode", unicodeIcons},
		{"none", noneIcons},
	}

	for _, set := range sets {
		t.Run(set.name, func(t *testing.T) {
			// Shuffle, RepeatAll, RepeatOne should always have values
			if set.icons.Shuffle == "" {
				t.Error("Shuffle icon should not be empty")
			}
			if set.icons.RepeatAll == "" {
				t.Error("RepeatAll icon should not be empty")
			}
			if set.icons.RepeatOne == "" {
				t.Error("RepeatOne icon should not be empty")
			}
			if set.icons.Favorite == "" {
				t.Error("Favorite icon should not be empty")
			}
			if set.icons.Radio == "" {
				t.Error("Radio icon should not be empty")
			}
		})
	}
}

func TestNerdIconsContainNerdFonts(t *testing.T) {
	Init("nerd")

	// Nerd fonts use private use area characters
	// Common ranges: \ue000-\uf8ff, \U000f0000-\U000fffff
	icons := []struct {
		name  string
		value string
	}{
		{"Folder", Folder()},
		{"Shuffle", Shuffle()},
		{"RepeatAll", RepeatAll()},
		{"RepeatOne", RepeatOne()},
		{"Favorite", Favorite()},
		{"Radio", Radio()},
	}

	for _, icon := range icons {
		t.Run(icon.name, func(t *testing.T) {
			// Just verify it's not ASCII-only
			hasNonASCII := false
			for _, r := range icon.value {
				if r > 127 {
					hasNonASCII = true
					break
				}
			}
			if !hasNonASCII {
				t.Errorf("%s icon should contain non-ASCII characters for nerd style, got %q", icon.name, icon.value)
			}
		})
	}

	Init("none")
}

func TestUnicodeIconsContainEmoji(t *testing.T) {
	Init("unicode")

	// Unicode icons should contain emoji or special characters
	icons := []struct {
		name  string
		value string
	}{
		{"Folder", Folder()},
		{"Shuffle", Shuffle()},
		{"RepeatAll", RepeatAll()},
		{"RepeatOne", RepeatOne()},
		{"Favorite", Favorite()},
		{"Radio", Radio()},
	}

	for _, icon := range icons {
		t.Run(icon.name, func(t *testing.T) {
			hasNonASCII := false
			for _, r := range icon.value {
				if r > 127 {
					hasNonASCII = true
					break
				}
			}
			if !hasNonASCII {
				t.Errorf("%s icon should contain non-ASCII characters for unicode style, got %q", icon.name, icon.value)
			}
		})
	}

	Init("none")
}

func TestNoneStyleUsesASCII(t *testing.T) {
	Init("none")

	// None style should use ASCII-only representations
	icons := []struct {
		name  string
		value string
	}{
		{"Folder", Folder()},
		{"Shuffle", Shuffle()},
		{"RepeatAll", RepeatAll()},
		{"RepeatOne", RepeatOne()},
		{"Favorite", Favorite()},
		{"Radio", Radio()},
	}

	for _, icon := range icons {
		t.Run(icon.name, func(t *testing.T) {
			for _, r := range icon.value {
				if r > 127 {
					t.Errorf("%s icon should only contain ASCII for none style, got %q", icon.name, icon.value)
					break
				}
			}
		})
	}
}

func TestFormatFunctionsWithSpecialCharacters(t *testing.T) {
	Init("unicode")

	// Test with various special characters in names
	specialNames := []string{
		"Name with spaces",
		"Name-with-dashes",
		"Name_with_underscores",
		"Name (with parentheses)",
		"Name [with brackets]",
		"日本語の名前",
		"Émojis 🎵 in name",
	}

	for _, name := range specialNames {
		t.Run("FormatDir_"+name, func(t *testing.T) {
			result := FormatDir(name)
			if !strings.Contains(result, name) {
				t.Errorf("FormatDir should contain original name, got %q", result)
			}
		})

		t.Run("FormatArtist_"+name, func(t *testing.T) {
			result := FormatArtist(name)
			if !strings.Contains(result, name) {
				t.Errorf("FormatArtist should contain original name, got %q", result)
			}
		})
	}

	Init("none")
}
