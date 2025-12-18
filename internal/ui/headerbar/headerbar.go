// internal/ui/headerbar/headerbar.go
package headerbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/styles"
)

// Height is the fixed height of the header bar (content + border).
const Height = 3

const (
	waveSymbol = "〰"
	logoText   = "WAVES"
	diag       = "╱"
	minDiags   = 3
)

// tab represents a header bar tab.
type tab struct {
	key  string
	name string
	mode string
}

// baseTabs are the always-available view tabs.
var baseTabs = []tab{
	{"F1", "Library", "library"},
	{"F2", "Files", "file"},
	{"F3", "Playlists", "playlists"},
}

// downloadsTab is shown when slskd is configured.
var downloadsTab = tab{"F4", "Downloads", "downloads"}

// LibrarySubMode represents which library view mode is active.
type LibrarySubMode int

const (
	LibraryModeMiller LibrarySubMode = iota
	LibraryModeAlbum
)

// Render returns the header bar string for the given width.
// currentMode should be "library", "file", "playlists", or "downloads".
// showDownloads controls whether the F4 Downloads tab is shown.
// librarySubMode indicates which library sub-mode is active (only shown when in library view).
func Render(currentMode string, width int, showDownloads bool, librarySubMode LibrarySubMode) string {
	if width < 20 {
		return ""
	}

	t := styles.T()

	// Border takes 2 chars on each side
	innerWidth := width - 2

	// Build logo: ~ WAVES (with gradient)
	logo := lipgloss.NewStyle().Foreground(t.Secondary).Render("~") +
		" " +
		styles.ApplyBoldGradient(logoText, t.Secondary, t.Primary)

	// Build tabs section
	tabsContent := renderTabs(currentMode, showDownloads, librarySubMode)

	// Calculate widths using lipgloss (handles ANSI codes correctly)
	logoWidth := lipgloss.Width(logo)
	tabsWidth := lipgloss.Width(tabsContent)
	usedWidth := logoWidth + 1 + tabsWidth  // logo + space + tabs
	fillWidth := innerWidth - usedWidth - 1 // -1 for trailing space before tabs

	// Build the header content
	var b strings.Builder
	b.WriteString(logo)
	b.WriteString(" ")

	if fillWidth >= minDiags {
		fill := lipgloss.NewStyle().Foreground(t.Primary).Render(strings.Repeat(diag, fillWidth))
		b.WriteString(fill)
		b.WriteString(" ")
	} else {
		// Not enough room for diagonals, just use remaining space
		remaining := innerWidth - logoWidth - 1 - tabsWidth
		if remaining > 0 {
			b.WriteString(strings.Repeat(" ", remaining))
		}
	}

	b.WriteString(tabsContent)

	// Wrap in border
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Width(innerWidth)

	return borderStyle.Render(b.String())
}

func renderTabs(currentMode string, showDownloads bool, librarySubMode LibrarySubMode) string {
	t := styles.T()

	// Build tab list
	tabs := baseTabs
	if showDownloads {
		tabs = append(tabs, downloadsTab)
	}

	parts := make([]string, 0, len(tabs))

	for _, tab := range tabs {
		isActive := tab.mode == currentMode

		var keyStyle, nameStyle lipgloss.Style
		if isActive {
			keyStyle = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
			nameStyle = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
		} else {
			keyStyle = t.S().Muted
			nameStyle = t.S().Base
		}

		part := keyStyle.Render(tab.key) + " " + nameStyle.Render(tab.name)

		// Add mode indicator for library tab when active (using dot separator)
		if tab.mode == "library" && isActive {
			var modeName string
			if librarySubMode == LibraryModeAlbum {
				modeName = "Albums"
			} else {
				modeName = "Browse"
			}
			part += t.S().Subtle.Render(" • ") + t.S().Muted.Render(modeName)
		}

		parts = append(parts, part)
	}

	separator := t.S().Subtle.Render(" │ ")
	content := strings.Join(parts, separator)

	// Help indicator: " │ ? Help"
	helpIndicator := separator + t.S().Muted.Render("?") + " " + t.S().Base.Render("Help")

	return content + helpIndicator
}
