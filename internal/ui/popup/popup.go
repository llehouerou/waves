package popup

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/llehouerou/waves/internal/ui/styles"
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
	t := styles.T()
	return Style{
		Border:      lipgloss.RoundedBorder(),
		BorderColor: t.Border,
		TitleStyle:  t.S().Title,
		FooterStyle: t.S().Subtle,
	}
}

// Dialog represents a simple centered popup with title, content, and footer.
type Dialog struct {
	Title   string
	Content string
	Footer  string
	Width   int // 0 = auto-fit content
	Height  int // 0 = auto-fit content
	Style   Style
}

// New creates a new dialog with default style.
func New() *Dialog {
	return &Dialog{
		Style: DefaultStyle(),
	}
}

// Render returns the dialog as a string ready to be overlaid.
// termWidth and termHeight are the terminal dimensions for centering.
func (p *Dialog) Render(termWidth, termHeight int) string {
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
		lines = append(lines, centerLine(titleText, innerWidth), "")
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
		lines = append(lines, "")
		footerText := style.FooterStyle.Render(p.Footer)
		lines = append(lines, centerLine(footerText, innerWidth))
	}

	// Apply border and padding
	content := strings.Join(lines, "\n")
	boxStyle := lipgloss.NewStyle().
		Border(style.Border).
		BorderForeground(style.BorderColor).
		Padding(0, 1).
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

// SizeConfig defines how a popup should be sized.
type SizeConfig struct {
	WidthPct  int // Percentage of screen width (0 = auto-fit)
	HeightPct int // Percentage of screen height (0 = auto-fit)
	MaxWidth  int // Maximum width in columns (0 = no limit)
}

// Common size configurations.
var (
	SizeLarge = SizeConfig{WidthPct: 80, HeightPct: 70} // Download, Import
	SizeAuto  = SizeConfig{}                            // Help, Confirm, etc.
)

// RenderBordered wraps content in a rounded border and centers it.
func RenderBordered(content string, screenW, screenH int, size SizeConfig) string {
	width, height := calculateDimensions(content, screenW, screenH, size)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.T().Border).
		Width(width-2). // Account for border
		Height(height-2).
		Padding(1, 2)

	box := boxStyle.Render(content)
	return Center(box, screenW, screenH)
}

func calculateDimensions(content string, screenW, screenH int, size SizeConfig) (width, height int) {
	if size.WidthPct > 0 {
		w := screenW * size.WidthPct / 100
		h := screenH * size.HeightPct / 100
		return w, h
	}
	// Auto-fit: calculate from content
	contentWidth := maxLineWidth(content)
	contentWidth += 6 // padding + border
	if size.MaxWidth > 0 && contentWidth > size.MaxWidth {
		contentWidth = size.MaxWidth
	}
	maxWidth := screenW - 4
	if contentWidth > maxWidth {
		contentWidth = maxWidth
	}

	contentHeight := strings.Count(content, "\n") + 1
	contentHeight += 4 // padding + border
	maxHeight := screenH - 4
	if contentHeight > maxHeight {
		contentHeight = maxHeight
	}

	return contentWidth, contentHeight
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

		// Find visible start position (count display columns of leading spaces)
		startCol := 0
		for _, r := range plainOverlay {
			if r != ' ' {
				break
			}
			startCol++ // ASCII space is always 1 column
		}

		// Calculate end position using display width
		trimmed := strings.TrimRight(plainOverlay, " ")
		endCol := ansi.StringWidth(trimmed)

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
		// When cutting through a wide character (like emoji), ansi.Cut may return
		// a shorter or longer string. We need to pad or trim to maintain alignment.
		prefix := ansi.Cut(baseLine, 0, startCol)
		prefixWidth := ansi.StringWidth(ansi.Strip(prefix))
		if prefixWidth < startCol {
			// Wide char was excluded from prefix - pad with spaces
			prefix += strings.Repeat(" ", startCol-prefixWidth)
		}

		result := prefix + overlayContent
		if endCol < width {
			suffix := ansi.Cut(baseLine, endCol, width)
			suffixPlain := ansi.Strip(suffix)
			suffixWidth := ansi.StringWidth(suffixPlain)
			expectedSuffixWidth := width - endCol
			if suffixWidth > expectedSuffixWidth {
				// Wide char was included in suffix but shouldn't be fully visible
				// Replace the first char (the wide char) with a space and trim
				// Use ansi.Cut to skip the extra width at the start
				suffix = " " + ansi.Cut(suffix, suffixWidth-expectedSuffixWidth+1, suffixWidth)
			} else if suffixWidth < expectedSuffixWidth {
				// Pad if suffix is too short
				result += strings.Repeat(" ", expectedSuffixWidth-suffixWidth)
			}
			result += suffix
		}

		baseLines[i] = result
	}

	return strings.Join(baseLines, "\n")
}
