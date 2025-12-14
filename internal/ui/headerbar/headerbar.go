// internal/ui/headerbar/headerbar.go
package headerbar

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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

// Styles
var (
	activeKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	activeNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	inactiveKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	inactiveNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// Render returns the header bar string for the given width.
// currentMode should be "library", "file", "playlists", or "downloads".
// showDownloads controls whether the F4 Downloads tab is shown.
func Render(currentMode string, width int, showDownloads bool) string {
	if width < 20 {
		return ""
	}

	// Build tab list
	tabs := baseTabs
	if showDownloads {
		tabs = append(tabs, downloadsTab)
	}

	parts := make([]string, 0, len(tabs))
	separator := separatorStyle.Render(" │ ")

	for _, t := range tabs {
		isActive := t.mode == currentMode

		var keyStyle, nameStyle lipgloss.Style
		if isActive {
			keyStyle = activeKeyStyle
			nameStyle = activeNameStyle
		} else {
			keyStyle = inactiveKeyStyle
			nameStyle = inactiveNameStyle
		}

		part := keyStyle.Render(t.key) + " " + nameStyle.Render(t.name)
		parts = append(parts, part)
	}

	content := strings.Join(parts, separator)

	// Center the content
	contentWidth := lipgloss.Width(content)
	if contentWidth < width {
		padLeft := (width - contentWidth) / 2
		content = strings.Repeat(" ", padLeft) + content
	}

	return content
}
