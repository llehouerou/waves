package styles

import (
	"testing"

	"github.com/llehouerou/waves/internal/config"
)

const (
	testRed   = "#ff0000"
	testGreen = "#00ff00"
)

func TestValidateHexColor(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		value   string
		wantErr bool
	}{
		{"valid 6-digit", "accent", testRed, false},
		{"valid 3-digit", "accent", "#f00", false},
		{"valid lowercase", "accent", "#abcdef", false},
		{"valid uppercase", "accent", "#ABCDEF", false},
		{"valid mixed case", "accent", "#aBcDeF", false},
		{"missing hash", "accent", "ff0000", true},
		{"too short", "accent", "#ff00", true},
		{"too long", "accent", "#ff00000", true},
		{"invalid chars", "accent", "#gggggg", true},
		{"empty", "accent", "", true},
		{"just hash", "accent", "#", true},
		{"not hex at all", "border", "red", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHexColor(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateHexColor(%q, %q) error = %v, wantErr %v", tt.field, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestExpandHex(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"#abc", "#aabbcc"},
		{"#ABC", "#AABBCC"},
		{"#f00", testRed},
		{"#abcdef", "#abcdef"},
		{"#ABCDEF", "#ABCDEF"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandHex(tt.input)
			if got != tt.want {
				t.Errorf("expandHex(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewTheme_EmptyConfig(t *testing.T) {
	theme, err := NewTheme(config.ThemeConfig{})
	if err != nil {
		t.Fatalf("NewTheme() error = %v", err)
	}

	def := defaultTheme
	if theme.Primary != def.Primary {
		t.Errorf("Primary = %q, want %q", theme.Primary, def.Primary)
	}
	if theme.Secondary != def.Secondary {
		t.Errorf("Secondary = %q, want %q", theme.Secondary, def.Secondary)
	}
	if theme.FgBase != def.FgBase {
		t.Errorf("FgBase = %q, want %q", theme.FgBase, def.FgBase)
	}
	if theme.FgMuted != def.FgMuted {
		t.Errorf("FgMuted = %q, want %q", theme.FgMuted, def.FgMuted)
	}
	if theme.BgBase != def.BgBase {
		t.Errorf("BgBase = %q, want %q", theme.BgBase, def.BgBase)
	}
	if theme.Border != def.Border {
		t.Errorf("Border = %q, want %q", theme.Border, def.Border)
	}
	if theme.BorderFocus != def.BorderFocus {
		t.Errorf("BorderFocus = %q, want %q", theme.BorderFocus, def.BorderFocus)
	}
	if theme.Success != def.Success {
		t.Errorf("Success = %q, want %q", theme.Success, def.Success)
	}
	if theme.Error != def.Error {
		t.Errorf("Error = %q, want %q", theme.Error, def.Error)
	}
	if theme.Warning != def.Warning {
		t.Errorf("Warning = %q, want %q", theme.Warning, def.Warning)
	}
	// FgSubtle and BgCursor are derived via HCL blending from the default
	// values, so they may differ slightly from the hardcoded defaults.
	// Just check they are non-empty.
	if theme.FgSubtle == "" {
		t.Error("FgSubtle should not be empty")
	}
	if theme.BgCursor == "" {
		t.Error("BgCursor should not be empty")
	}
}

func TestNewTheme_AllOverrides(t *testing.T) {
	accent := testRed
	secondary := testGreen
	text := "#ffffff"
	muted := "#888888"
	bg := "#000000"
	border := "#444444"

	theme, err := NewTheme(config.ThemeConfig{
		Accent:     &accent,
		Secondary:  &secondary,
		Text:       &text,
		Muted:      &muted,
		Background: &bg,
		Border:     &border,
	})
	if err != nil {
		t.Fatalf("NewTheme() error = %v", err)
	}

	// Direct mappings
	if theme.Primary != testRed {
		t.Errorf("Primary = %q, want %s", theme.Primary, testRed)
	}
	if theme.Secondary != testGreen {
		t.Errorf("Secondary = %q, want %s", theme.Secondary, testGreen)
	}
	if theme.FgBase != "#ffffff" {
		t.Errorf("FgBase = %q, want #ffffff", theme.FgBase)
	}
	if theme.FgMuted != "#888888" {
		t.Errorf("FgMuted = %q, want #888888", theme.FgMuted)
	}
	if theme.BgBase != "#000000" {
		t.Errorf("BgBase = %q, want #000000", theme.BgBase)
	}
	if theme.Border != "#444444" {
		t.Errorf("Border = %q, want #444444", theme.Border)
	}
	if theme.BorderFocus != testRed {
		t.Errorf("BorderFocus = %q, want %s", theme.BorderFocus, testRed)
	}
	if theme.Warning != testGreen {
		t.Errorf("Warning = %q, want %s", theme.Warning, testGreen)
	}
	if theme.Success != defaultTheme.Success {
		t.Errorf("Success = %q, want %q", theme.Success, defaultTheme.Success)
	}
	if theme.Error != defaultTheme.Error {
		t.Errorf("Error = %q, want %q", theme.Error, defaultTheme.Error)
	}
	// Derived: FgSubtle should be between muted and background
	if theme.FgSubtle == theme.FgMuted || theme.FgSubtle == theme.BgBase {
		t.Errorf("FgSubtle = %q, expected a blend between muted and background", theme.FgSubtle)
	}
	// Derived: BgCursor should be between background and text
	if theme.BgCursor == theme.BgBase || theme.BgCursor == theme.FgBase {
		t.Errorf("BgCursor = %q, expected a blend between background and text", theme.BgCursor)
	}
}

func TestNewTheme_SingleOverride(t *testing.T) {
	accent := testRed
	theme, err := NewTheme(config.ThemeConfig{Accent: &accent})
	if err != nil {
		t.Fatalf("NewTheme() error = %v", err)
	}

	if theme.Primary != testRed {
		t.Errorf("Primary = %q, want %s", theme.Primary, testRed)
	}
	if theme.BorderFocus != testRed {
		t.Errorf("BorderFocus = %q, want %s", theme.BorderFocus, testRed)
	}
	if theme.Secondary != defaultTheme.Secondary {
		t.Errorf("Secondary = %q, want default %q", theme.Secondary, defaultTheme.Secondary)
	}
	if theme.FgBase != defaultTheme.FgBase {
		t.Errorf("FgBase = %q, want default %q", theme.FgBase, defaultTheme.FgBase)
	}
}

func TestNewTheme_ShortHex(t *testing.T) {
	accent := "#f00"
	theme, err := NewTheme(config.ThemeConfig{Accent: &accent})
	if err != nil {
		t.Fatalf("NewTheme() error = %v", err)
	}

	if theme.Primary != testRed {
		t.Errorf("Primary = %q, want %s", theme.Primary, testRed)
	}
}

func TestNewTheme_InvalidColor(t *testing.T) {
	bad := "not-a-color"
	_, err := NewTheme(config.ThemeConfig{Accent: &bad})
	if err == nil {
		t.Error("NewTheme() expected error for invalid color, got nil")
	}
}
