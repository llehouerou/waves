// Package helpbindings provides a scrollable popup for displaying keybindings.
package helpbindings

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// Compile-time check that Model implements popup.Popup.
var _ popup.Popup = (*Model)(nil)

// categoryOrder defines the display order of binding categories.
var categoryOrder = []string{
	"global",
	"playback",
	"navigator",
	"filebrowser",
	"library",
	"albumview",
	"queue",
	"playlist",
	"playlist-track",
}

// categoryLabels maps context names to display labels.
var categoryLabels = map[string]string{
	"global":         "Global",
	"playback":       "Playback",
	"navigator":      "Navigator",
	"filebrowser":    "File Browser",
	"library":        "Library",
	"albumview":      "Album View",
	"queue":          "Queue Panel",
	"playlist":       "Playlist",
	"playlist-track": "Playlist Tracks",
}

// Model holds the state for the help bindings popup.
type Model struct {
	ui.Base
	bindings     []keymap.Binding
	contexts     []string
	scrollOffset int
}

// New creates a new help bindings model.
func New() Model {
	return Model{}
}

// SetContexts sets which binding contexts to display.
func (m *Model) SetContexts(contexts []string) {
	m.contexts = contexts
	m.bindings = nil
	for _, ctx := range categoryOrder {
		if slices.Contains(contexts, ctx) {
			m.bindings = append(m.bindings, keymap.ByContext(ctx)...)
		}
	}
	m.scrollOffset = 0
}

// Init implements popup.Popup.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements popup.Popup.
func (m *Model) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
	switch key {
	case "?", "esc", "q":
		return m, func() tea.Msg { return ActionMsg(Close{}) }
	case "j", "down":
		maxScroll := m.maxScroll()
		if m.scrollOffset < maxScroll {
			m.scrollOffset++
		}
	case "k", "up":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	}
	return m, nil
}

// View implements popup.Popup.
func (m *Model) View() string {
	return m.render()
}

// render renders the help popup content (without border - popup manager adds that).
func (m *Model) render() string {
	if m.Width() == 0 || m.Height() == 0 {
		return ""
	}

	content := m.buildContent()
	lines := strings.Split(content, "\n")

	// Calculate max width from ALL lines (not just visible) for consistent popup width
	maxWidth := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxWidth {
			maxWidth = w
		}
	}

	// Calculate visible area
	visibleHeight := m.visibleHeight()
	if visibleHeight <= 0 {
		visibleHeight = len(lines)
	}

	// Apply scroll offset
	startLine := m.scrollOffset
	endLine := min(startLine+visibleHeight, len(lines))
	startLine = min(startLine, len(lines))

	visibleLines := lines[startLine:endLine]

	// Pad visible lines to max width for consistent popup sizing
	for i, line := range visibleLines {
		if w := lipgloss.Width(line); w < maxWidth {
			visibleLines[i] = line + strings.Repeat(" ", maxWidth-w)
		}
	}

	// Build content with title and footer
	titleStyle := lipgloss.NewStyle().Bold(true)
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	var result strings.Builder
	result.WriteString(titleStyle.Render("Help"))
	result.WriteString("\n\n")
	result.WriteString(strings.Join(visibleLines, "\n"))
	result.WriteString("\n\n")
	result.WriteString(footerStyle.Render(m.buildFooter()))

	return result.String()
}

func (m Model) buildContent() string {
	var sb strings.Builder

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Find max key width for alignment
	maxKeyWidth := 0
	for _, b := range m.bindings {
		keyStr := strings.Join(b.Keys, ", ")
		if len(keyStr) > maxKeyWidth {
			maxKeyWidth = len(keyStr)
		}
	}

	currentContext := ""
	for _, b := range m.bindings {
		// Add category header when context changes
		if b.Context != currentContext {
			if currentContext != "" {
				sb.WriteString("\n")
			}
			label := categoryLabels[b.Context]
			if label == "" {
				label = b.Context
			}
			sb.WriteString(headerStyle.Render(label))
			sb.WriteString("\n")
			sb.WriteString(separatorStyle.Render(strings.Repeat("─", maxKeyWidth+15)))
			sb.WriteString("\n")
			currentContext = b.Context
		}

		// Render key binding
		keyStr := strings.Join(b.Keys, ", ")
		paddedKey := keyStr + strings.Repeat(" ", maxKeyWidth-len(keyStr))
		sb.WriteString(keyStyle.Render(paddedKey))
		sb.WriteString("  ")
		sb.WriteString(descStyle.Render(b.Description))
		sb.WriteString("\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

func (m Model) buildFooter() string {
	totalLines := m.totalLines()
	visibleHeight := m.visibleHeight()

	if totalLines <= visibleHeight {
		return "?/esc close"
	}

	return "j/k scroll · ?/esc close"
}

func (m Model) visibleHeight() int {
	// Leave room for popup chrome (title, footer, borders, margins)
	return max(m.Height()-10, 5)
}

func (m Model) totalLines() int {
	content := m.buildContent()
	return strings.Count(content, "\n") + 1
}

func (m Model) maxScroll() int {
	total := m.totalLines()
	visible := m.visibleHeight()
	if total <= visible {
		return 0
	}
	return total - visible
}
