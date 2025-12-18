package styles

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/rivo/uniseg"
)

// ApplyGradient renders text with a horizontal color gradient.
func ApplyGradient(text string, from, to lipgloss.Color) string {
	return applyGradient(text, false, from, to)
}

// ApplyBoldGradient renders bold text with a horizontal color gradient.
func ApplyBoldGradient(text string, from, to lipgloss.Color) string {
	return applyGradient(text, true, from, to)
}

func applyGradient(text string, bold bool, from, to lipgloss.Color) string {
	if text == "" {
		return ""
	}

	// Split into grapheme clusters for proper unicode handling
	var clusters []string
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		clusters = append(clusters, gr.Str())
	}

	if len(clusters) == 0 {
		return ""
	}

	if len(clusters) == 1 {
		style := lipgloss.NewStyle().Foreground(from)
		if bold {
			style = style.Bold(true)
		}
		return style.Render(text)
	}

	// Blend colors across the text
	colors := blendColors(len(clusters), from, to)

	var b strings.Builder
	for i, cluster := range clusters {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colorToHex(colors[i])))
		if bold {
			style = style.Bold(true)
		}
		b.WriteString(style.Render(cluster))
	}

	return b.String()
}

// blendColors returns a slice of colors blended between from and to.
// Blending is done in HCL color space for perceptually uniform transitions.
func blendColors(size int, from, to lipgloss.Color) []color.Color {
	if size < 2 {
		return []color.Color{from}
	}

	c1, _ := colorful.MakeColor(lipglossToColor(from))
	c2, _ := colorful.MakeColor(lipglossToColor(to))

	colors := make([]color.Color, size)
	for i := range size {
		t := float64(i) / float64(size-1)
		colors[i] = c1.BlendHcl(c2, t)
	}

	return colors
}

// lipglossToColor converts a lipgloss.Color to a color.Color.
func lipglossToColor(c lipgloss.Color) color.Color {
	hex := string(c)
	if len(hex) == 7 && hex[0] == '#' {
		col, err := colorful.Hex(hex)
		if err == nil {
			return col
		}
	}
	// Fallback for ANSI colors - return a neutral gray
	return color.RGBA{R: 128, G: 128, B: 128, A: 255}
}

// colorToHex converts a color.Color to a hex string.
func colorToHex(c color.Color) string {
	cf, ok := c.(colorful.Color)
	if ok {
		return cf.Hex()
	}
	r, g, b, _ := c.RGBA()
	return colorful.Color{
		R: float64(r) / 65535.0,
		G: float64(g) / 65535.0,
		B: float64(b) / 65535.0,
	}.Hex()
}
