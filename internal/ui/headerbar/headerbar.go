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

// tabs are the available view tabs.
var tabs = []tab{
	{"F1", "Library", "library"},
	{"F2", "Files", "file"},
	{"F3", "Playlists", "playlists"},
}

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
// currentMode should be "library", "file", or "playlists".
func Render(currentMode string, width int) string {
	if width < 20 {
		return ""
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
