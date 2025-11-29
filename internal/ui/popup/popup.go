package popup

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Style configures the popup appearance.
type Style struct {
	Border      lipgloss.Border
	BorderColor lipgloss.Color
	TitleStyle  lipgloss.Style
	FooterStyle lipgloss.Style
}

// DefaultStyle returns the default popup style.
func DefaultStyle() Style {
	return Style{
		Border:      lipgloss.RoundedBorder(),
		BorderColor: lipgloss.Color("240"),
		TitleStyle:  lipgloss.NewStyle().Bold(true),
		FooterStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	}
}

// Popup represents a centered popup overlay.
type Popup struct {
	Title   string
	Content string
	Footer  string
	Width   int // 0 = auto-fit content
	Height  int // 0 = auto-fit content
	Style   Style
}

// New creates a new popup with default style.
func New() *Popup {
	return &Popup{
		Style: DefaultStyle(),
	}
}

// Render returns the popup as a string ready to be overlaid.
// termWidth and termHeight are the terminal dimensions for centering.
func (p *Popup) Render(termWidth, termHeight int) string {
	style := p.Style

	// Calculate content width
	contentWidth := p.Width
	if contentWidth == 0 {
		// Auto-fit: find widest line
		contentWidth = maxLineWidth(p.Content)
		if p.Title != "" && len(p.Title) > contentWidth {
			contentWidth = len(p.Title)
		}
		if p.Footer != "" && len(p.Footer) > contentWidth {
			contentWidth = len(p.Footer)
		}
		contentWidth += 2 // padding
	}

	// Limit to terminal width
	maxWidth := termWidth - 4
	if contentWidth > maxWidth {
		contentWidth = maxWidth
	}

	innerWidth := contentWidth

	// Build popup content - estimate capacity
	contentLineCount := strings.Count(p.Content, "\n") + 1
	capacity := contentLineCount + 4 // title, separators, footer
	lines := make([]string, 0, capacity)

	// Title
	if p.Title != "" {
		titleText := style.TitleStyle.Render(p.Title)
		lines = append(lines, centerLine(titleText, innerWidth), strings.Repeat("─", innerWidth))
	}

	// Content
	for line := range strings.SplitSeq(p.Content, "\n") {
		// Truncate if needed
		if lipgloss.Width(line) > innerWidth {
			line = line[:innerWidth-3] + "..."
		}
		lines = append(lines, padLine(line, innerWidth))
	}

	// Footer
	if p.Footer != "" {
		lines = append(lines, strings.Repeat("─", innerWidth))
		footerText := style.FooterStyle.Render(p.Footer)
		lines = append(lines, centerLine(footerText, innerWidth))
	}

	// Apply border
	content := strings.Join(lines, "\n")
	boxStyle := lipgloss.NewStyle().
		Border(style.Border).
		BorderForeground(style.BorderColor).
		Width(innerWidth)

	box := boxStyle.Render(content)

	// Center in terminal
	return centerBox(box, termWidth, termHeight)
}

func maxLineWidth(s string) int {
	maxW := 0
	for line := range strings.SplitSeq(s, "\n") {
		w := lipgloss.Width(line)
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func centerLine(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	pad := (width - w) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-w-pad)
}

func padLine(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// Center centers pre-rendered content in the terminal.
// Useful when you have custom-styled content that just needs centering.
func Center(content string, termWidth, termHeight int) string {
	return centerBox(content, termWidth, termHeight)
}

func centerBox(box string, termWidth, termHeight int) string {
	lines := strings.Split(box, "\n")
	boxHeight := len(lines)
	boxWidth := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > boxWidth {
			boxWidth = w
		}
	}

	padTop := (termHeight - boxHeight) / 2
	padLeft := (termWidth - boxWidth) / 2

	if padTop < 0 {
		padTop = 0
	}
	if padLeft < 0 {
		padLeft = 0
	}

	var result strings.Builder
	for range padTop {
		result.WriteString(strings.Repeat(" ", termWidth) + "\n")
	}
	for _, line := range lines {
		result.WriteString(strings.Repeat(" ", padLeft))
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}

// Compose overlays content on top of a base view.
// Non-space characters in overlay replace the base at the same position.
// This function is ANSI-aware and handles styled text correctly.
func Compose(base, popupView string, width, _ int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(popupView, "\n")

	for i, overlayLine := range overlayLines {
		if i >= len(baseLines) {
			break
		}

		// Strip ANSI to find visible content bounds
		plainOverlay := ansi.Strip(overlayLine)
		if strings.TrimSpace(plainOverlay) == "" {
			continue // empty line (visually)
		}

		// Find visible start and end positions (in display columns)
		startCol := 0
		for _, r := range plainOverlay {
			if r != ' ' {
				break
			}
			startCol++
		}

		// Trim trailing spaces from end position
		trimmed := strings.TrimRight(plainOverlay, " ")
		endCol := startCol + ansi.StringWidth(trimmed[startCol:])

		// Extract the overlay content (with ANSI codes intact)
		overlayContent := ansi.Cut(overlayLine, startCol, endCol)

		// Build new line: base prefix + overlay content + base suffix
		baseLine := baseLines[i]
		baseWidth := ansi.StringWidth(ansi.Strip(baseLine))

		// Pad base line if needed
		if baseWidth < width {
			baseLine += strings.Repeat(" ", width-baseWidth)
		}

		// Construct result: base[0:startCol] + overlay + base[endCol:]
		result := ansi.Cut(baseLine, 0, startCol) + overlayContent
		if endCol < width {
			result += ansi.Cut(baseLine, endCol, width)
		}

		baseLines[i] = result
	}

	return strings.Join(baseLines, "\n")
}
