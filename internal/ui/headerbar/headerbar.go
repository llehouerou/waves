// internal/ui/headerbar/headerbar.go
package headerbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/ui/styles"
)

// Height is the fixed height of the header bar (single line).
const Height = 1

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

func activeKeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.T().Primary).
		Bold(true)
}

func activeNameStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.T().Primary).
		Bold(true)
}

func inactiveKeyStyle() lipgloss.Style {
	return styles.T().S().Muted
}

func inactiveNameStyle() lipgloss.Style {
	return styles.T().S().Base
}

func separatorStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

// LibrarySubMode represents which library view mode is active.
type LibrarySubMode int

const (
	LibraryModeMiller LibrarySubMode = iota
	LibraryModeAlbum
)

func subModeStyle() lipgloss.Style {
	return styles.T().S().Muted
}

// Render returns the header bar string for the given width.
// currentMode should be "library", "file", "playlists", or "downloads".
// showDownloads controls whether the F4 Downloads tab is shown.
// librarySubMode indicates which library sub-mode is active (only shown when in library view).
func Render(currentMode string, width int, showDownloads bool, librarySubMode LibrarySubMode) string {
	if width < 20 {
		return ""
	}

	// Build tab list
	tabs := baseTabs
	if showDownloads {
		tabs = append(tabs, downloadsTab)
	}

	parts := make([]string, 0, len(tabs))
	separator := separatorStyle().Render(" │ ")

	for _, t := range tabs {
		isActive := t.mode == currentMode

		var keyStyle, nameStyle lipgloss.Style
		if isActive {
			keyStyle = activeKeyStyle()
			nameStyle = activeNameStyle()
		} else {
			keyStyle = inactiveKeyStyle()
			nameStyle = inactiveNameStyle()
		}

		part := keyStyle.Render(t.key) + " " + nameStyle.Render(t.name)

		// Add mode indicator for library tab when active
		if t.mode == "library" && isActive {
			var modeName string
			if librarySubMode == LibraryModeAlbum {
				modeName = "Albums"
			} else {
				modeName = "Browse"
			}
			part += " " + subModeStyle().Render("("+modeName+")")
		}

		parts = append(parts, part)
	}

	content := strings.Join(parts, separator)

	// Help indicator on the right
	helpIndicator := inactiveKeyStyle().Render("?") + " " + inactiveNameStyle().Render("Help")
	helpWidth := lipgloss.Width(helpIndicator)

	// Center the tabs content, then place help on the right
	contentWidth := lipgloss.Width(content)
	if contentWidth < width {
		padLeft := (width - contentWidth) / 2
		content = strings.Repeat(" ", padLeft) + content
	}

	// Calculate remaining space and add help indicator on the right
	currentWidth := lipgloss.Width(content)
	remainingSpace := width - currentWidth - helpWidth
	if remainingSpace > 0 {
		content = content + strings.Repeat(" ", remainingSpace) + helpIndicator
	}

	return content
}
