package styles

import (
	"fmt"
	"regexp"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/llehouerou/waves/internal/config"
)

var hexColorRe = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func validateHexColor(field, value string) error {
	if !hexColorRe.MatchString(value) {
		return fmt.Errorf(
			"invalid theme color %q: %q is not a valid hex color (#RGB or #RRGGBB)",
			field, value,
		)
	}
	return nil
}

// expandHex converts short hex (#RGB) to full hex (#RRGGBB).
// Assumes input is already validated.
func expandHex(hex string) string {
	if len(hex) == 4 {
		return "#" + string(hex[1]) + string(hex[1]) +
			string(hex[2]) + string(hex[2]) +
			string(hex[3]) + string(hex[3])
	}
	return hex
}

// NewTheme creates a Theme from user config, applying defaults and deriving
// technical colors. Returns an error if any provided color is invalid.
func NewTheme(cfg config.ThemeConfig) (*Theme, error) {
	t := defaultTheme
	t.styles = nil // reset cached styles

	type override struct {
		field string
		value *string
		apply func(lipgloss.Color)
	}

	overrides := []override{
		{"accent", cfg.Accent, func(c lipgloss.Color) { t.Primary = c; t.BorderFocus = c }},
		{"secondary", cfg.Secondary, func(c lipgloss.Color) { t.Secondary = c; t.Warning = c }},
		{"text", cfg.Text, func(c lipgloss.Color) { t.FgBase = c }},
		{"muted", cfg.Muted, func(c lipgloss.Color) { t.FgMuted = c }},
		{"background", cfg.Background, func(c lipgloss.Color) { t.BgBase = c }},
		{"border", cfg.Border, func(c lipgloss.Color) { t.Border = c }},
	}

	for _, o := range overrides {
		if o.value == nil {
			continue
		}
		if err := validateHexColor(o.field, *o.value); err != nil {
			return nil, err
		}
		o.apply(lipgloss.Color(expandHex(*o.value)))
	}

	// Derive FgSubtle only if muted or background changed
	if cfg.Muted != nil || cfg.Background != nil {
		t.FgSubtle = blendHex(t.FgMuted, t.BgBase, 0.3)
	}

	// Derive BgCursor only if background or text changed
	if cfg.Background != nil || cfg.Text != nil {
		t.BgCursor = blendHex(t.BgBase, t.FgBase, 0.15)
	}

	return &t, nil
}

// blendHex blends two lipgloss colors in HCL space and returns the result.
func blendHex(from, to lipgloss.Color, t float64) lipgloss.Color {
	c1, _ := colorful.Hex(string(from))
	c2, _ := colorful.Hex(string(to))
	blended := c1.BlendHcl(c2, t)
	return lipgloss.Color(blended.Clamped().Hex())
}
