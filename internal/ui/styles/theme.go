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
	base := lipgloss.NewStyle().Foreground(t.FgBase)

	return &Styles{
		Base:   base,
		Muted:  lipgloss.NewStyle().Foreground(t.FgMuted),
		Subtle: lipgloss.NewStyle().Foreground(t.FgSubtle),
		Title:  base.Bold(true),
		Playing: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),
		Cursor: lipgloss.NewStyle().
			Background(t.BgCursor).
			Foreground(t.FgBase),
		Success: lipgloss.NewStyle().Foreground(t.Success),
		Error:   lipgloss.NewStyle().Foreground(t.Error),
		Warning: lipgloss.NewStyle().Foreground(t.Warning),
	}
}
