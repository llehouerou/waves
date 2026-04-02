package styles

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette and pre-built styles for the application.
type Theme struct {
	// Brand/accent colors
	Primary   lipgloss.Color // Purple - focused items, active states
	Secondary lipgloss.Color // Gold/orange - secondary accent

	// Text hierarchy (most to least prominent)
	FgBase   lipgloss.Color // Primary text (bright)
	FgMuted  lipgloss.Color // Secondary text (dimmed)
	FgSubtle lipgloss.Color // Tertiary text (very dim)

	// Backgrounds
	BgBase   lipgloss.Color // Panel backgrounds
	BgCursor lipgloss.Color // Cursor/selection highlight

	// Borders
	Border      lipgloss.Color // Unfocused panel borders
	BorderFocus lipgloss.Color // Focused panel borders

	// Status colors
	Success lipgloss.Color // Green - added, playing
	Error   lipgloss.Color // Red - errors, removed
	Warning lipgloss.Color // Yellow/orange - warnings

	// HasExplicitBackground is true when the user explicitly set a background color.
	// When false, the terminal's native background is used.
	HasExplicitBackground bool

	styles *Styles
}

// Styles contains pre-built lipgloss styles for common UI patterns.
type Styles struct {
	Base    lipgloss.Style // Default text
	Muted   lipgloss.Style // Dimmed text
	Subtle  lipgloss.Style // Very dim text
	Title   lipgloss.Style // Bold, bright
	Playing lipgloss.Style // Currently playing track
	Cursor  lipgloss.Style // Cursor background highlight
	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
}

var defaultTheme = Theme{
	// Bright purple accent
	Primary:   lipgloss.Color("#a78bfa"),
	Secondary: lipgloss.Color("#f1a208"),

	// Text hierarchy (grayscale)
	FgBase:   lipgloss.Color("#c0c0c0"),
	FgMuted:  lipgloss.Color("#808080"),
	FgSubtle: lipgloss.Color("#585858"),

	// Backgrounds
	BgBase:   lipgloss.Color("#1a1a1a"),
	BgCursor: lipgloss.Color("#303030"),

	// Borders
	Border:      lipgloss.Color("#585858"),
	BorderFocus: lipgloss.Color("#a78bfa"),

	// Status
	Success: lipgloss.Color("#42b883"),
	Error:   lipgloss.Color("#ff5555"),
	Warning: lipgloss.Color("#f1a208"),
}

// T returns the default theme.
func T() *Theme {
	return &defaultTheme
}

// S returns the pre-built styles for this theme.
func (t *Theme) S() *Styles {
	if t.styles == nil {
		t.styles = t.buildStyles()
	}
	return t.styles
}

func (t *Theme) buildStyles() *Styles {
	bg := t.baseStyle()
	base := bg.Foreground(t.FgBase)

	return &Styles{
		Base:   base,
		Muted:  bg.Foreground(t.FgMuted),
		Subtle: bg.Foreground(t.FgSubtle),
		Title:  base.Bold(true),
		Playing: bg.
			Foreground(t.Primary).
			Bold(true),
		Cursor: bg.
			Background(t.BgCursor).
			Foreground(t.FgBase),
		Success: bg.Foreground(t.Success),
		Error:   bg.Foreground(t.Error),
		Warning: bg.Foreground(t.Warning),
	}
}

// baseStyle returns a style with the background set if the theme has an
// explicit background, or an empty style otherwise.
func (t *Theme) baseStyle() lipgloss.Style {
	s := lipgloss.NewStyle()
	if t.HasExplicitBackground {
		s = s.Background(t.BgBase)
	}
	return s
}

// BaseStyle returns a lipgloss.Style pre-configured with the theme background
// when an explicit background is set. Use this instead of lipgloss.NewStyle()
// in UI components to ensure consistent background rendering.
func (t *Theme) BaseStyle() lipgloss.Style {
	return t.baseStyle()
}

// Bg applies the theme background to unstyled text (spaces, separators, etc.)
// when an explicit background is set. Returns the text unchanged otherwise.
func (t *Theme) Bg(s string) string {
	if t.HasExplicitBackground {
		return t.baseStyle().Render(s)
	}
	return s
}
