package popup

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/llehouerou/waves/internal/ui/render"
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
	Style   Style
}

// New creates a new dialog with default style.
func New() *Dialog {
	return &Dialog{
		Style: DefaultStyle(),
	}
}

// Render returns the dialog as a string ready to be overlaid, centred in a
// terminal of termWidth×termHeight.
//
// The box is as wide as its widest line, at most the terminal width minus 2.
// Inside its border and its one column of padding on each side, the text area
// holds the title and the footer, centred and cut to it, and the content, each
// line wrapped to it. When the content has more lines than the terminal has
// room for, its last line shown ends with "...".
func (p *Dialog) Render(termWidth, termHeight int) string {
	style := p.Style

	// The border and the padding take 2 columns on each side of the text area,
	// and the box keeps 1 column of margin on each side of the terminal.
	textWidth := max(maxLineWidth(p.Content), ansi.StringWidth(p.Title), ansi.StringWidth(p.Footer))
	textWidth = max(min(textWidth, termWidth-6), 0)

	var content []string
	for line := range strings.SplitSeq(p.Content, "\n") {
		if ansi.StringWidth(line) > textWidth && textWidth > 0 {
			content = append(content, wrap(line, textWidth)...)
		} else {
			content = append(content, line)
		}
	}

	// Rows the box takes besides the content: its border, the title and the
	// footer with the blank line that sets each apart.
	chrome := 2
	if p.Title != "" {
		chrome += 2
	}
	if p.Footer != "" {
		chrome += 2
	}
	if room := max(termHeight-chrome, 1); len(content) > room {
		content = content[:room]
		content[room-1] = cut(content[room-1]+"...", textWidth)
	}

	lines := make([]string, 0, len(content)+chrome-2)
	if p.Title != "" {
		title := style.TitleStyle.Render(cut(p.Title, textWidth))
		lines = append(lines, centerLine(title, textWidth), "")
	}
	for _, line := range content {
		lines = append(lines, padLine(cut(line, textWidth), textWidth))
	}
	if p.Footer != "" {
		footer := style.FooterStyle.Render(cut(p.Footer, textWidth))
		lines = append(lines, "", centerLine(footer, textWidth))
	}

	t := styles.T()
	boxStyle := t.BaseStyle().
		Border(style.Border).
		BorderForeground(style.BorderColor).
		Padding(0, 1).
		Width(textWidth + 2) // lipgloss counts the padding in the width
	if t.HasExplicitBackground {
		boxStyle = boxStyle.BorderBackground(t.BgBase)
	}

	box := strings.Split(boxStyle.Render(strings.Join(lines, "\n")), "\n")
	// In a terminal too short for the box, what fits is drawn.
	box = box[:min(len(box), max(termHeight, 0))]
	return centerBox(strings.Join(box, "\n"), termWidth, termHeight)
}

// cut shortens s to width columns, ending with "..." when there is room for it.
func cut(s string, width int) string {
	if width < 3 {
		return ansi.Truncate(s, width, "")
	}
	return ansi.Truncate(s, width, "...")
}

// wrap breaks s into lines of at most width columns, at spaces. A word wider
// than a line on its own fills the line it starts on and goes on over the next.
func wrap(s string, width int) []string {
	var lines []string
	line := ""
	for word := range strings.FieldsSeq(s) {
		next := word
		if line != "" {
			next = line + " " + word
		}
		switch {
		case ansi.StringWidth(next) <= width:
			line = next
		case ansi.StringWidth(word) <= width:
			lines = append(lines, line)
			line = word
		default:
			parts := strings.Split(ansi.Hardwrap(next, width, false), "\n")
			lines = append(lines, parts[:len(parts)-1]...)
			line = parts[len(parts)-1]
		}
	}
	return append(lines, line)
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
	return render.EmptyLine(pad) + s + render.EmptyLine(width-w-pad)
}

func padLine(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + render.EmptyLine(width-w)
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
		result.WriteString(render.EmptyLine(termWidth) + "\n")
	}
	for _, line := range lines {
		result.WriteString(render.EmptyLine(padLeft))
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

	pt := styles.T()
	boxStyle := pt.BaseStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(pt.Border).
		Width(width-2). // Account for border
		Height(height-2).
		Padding(1, 2)
	if pt.HasExplicitBackground {
		boxStyle = boxStyle.BorderBackground(pt.BgBase)
	}

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
			baseLine += render.EmptyLine(width - baseWidth)
		}

		// Construct result: base[0:startCol] + overlay + base[endCol:]
		// When cutting through a wide character (like emoji), ansi.Cut may return
		// a shorter or longer string. We need to pad or trim to maintain alignment.
		prefix := ansi.Cut(baseLine, 0, startCol)
		prefixWidth := ansi.StringWidth(ansi.Strip(prefix))
		if prefixWidth < startCol {
			// Wide char was excluded from prefix - pad with spaces
			prefix += render.EmptyLine(startCol - prefixWidth)
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
				result += render.EmptyLine(expectedSuffixWidth - suffixWidth)
			}
			result += suffix
		}

		baseLines[i] = result
	}

	return strings.Join(baseLines, "\n")
}
