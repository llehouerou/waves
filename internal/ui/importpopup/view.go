package importpopup

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/keymap"
	"github.com/llehouerou/waves/internal/ui/fit"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// Symbols for status indicators
const (
	completedSymbol = "\u2714" // ✔
	failedSymbol    = "\u2717" // ✗
	progressSymbol  = "\u21E9" // ⇩
	pendingSymbol   = "\u25CB" // ○
)

// Separators
const (
	sepArrow = " \u2192 " // →
)

// keys gives the global keys the path preview names when there is no library
// source to import into.
var keys = keymap.NewResolver(keymap.ByContext(keymap.ContextGlobal))

// Layout thresholds
const (
	// compactWidthThreshold is the width below which compact layout is used.
	// Matches the threshold used in the download results view.
	compactWidthThreshold = 90
)

func titleStyle() lipgloss.Style {
	return styles.T().S().Title
}

func headerStyle() lipgloss.Style {
	return styles.T().S().Muted
}

func stepStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func activeStepStyle() lipgloss.Style {
	return styles.T().S().Playing.Bold(true)
}

func labelStyle() lipgloss.Style {
	return styles.T().S().Muted
}

func valueStyle() lipgloss.Style {
	return styles.T().S().Base
}

func changedStyle() lipgloss.Style {
	return styles.T().S().Warning
}

func successStyle() lipgloss.Style {
	return styles.T().S().Success
}

func errorStyle() lipgloss.Style {
	return styles.T().S().Error
}

func dimStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func selectedStyle() lipgloss.Style {
	return styles.T().S().Cursor
}

// View renders the import popup.
func (m *Model) View() string {
	if m.Width() == 0 || m.Height() == 0 {
		return ""
	}
	return m.page().String()
}

// page is what the view of the popup's state shows: its list scrolled where
// the user left it, or, while importing, where the import is.
func (m *Model) page() fit.Page {
	switch m.state {
	case StateTagPreview:
		return m.boxed(m.tagPreviewView()).At(m.tagOffset)
	case StatePathPreview:
		return m.boxed(m.pathPreviewView()).At(m.pathOffset)
	case StateImporting:
		return m.boxed(m.importingView()).Showing(m.followedFile())
	case StateComplete:
		return m.boxed(m.completeView()).At(m.failedOffset)
	}
	return fit.Page{}
}

// boxed is v in the box of the popup: as wide as the popup, and as tall as
// it may grow.
func (m *Model) boxed(v fit.View) fit.View {
	return m.Fit(v, m.Width(), m.Height())
}

// heading is what each view shows first: the title and the steps.
func (m *Model) heading() []fit.Line {
	title := fmt.Sprintf("Import: %s - %s", m.download.MBArtistName, m.download.MBAlbumTitle)
	return fit.Lines(
		titleStyle().Render(title),
		"",
		m.renderStepIndicator(),
		"",
		render.Separator(m.innerWidth()),
		"",
	)
}

// innerWidth returns the actual content width accounting for popup border and padding.
func (m *Model) innerWidth() int {
	// Popup has: 2 for border + 4 for padding (2 each side)
	return m.Width() - 8
}

// renderStepIndicator renders the step progress indicator.
func (m *Model) renderStepIndicator() string {
	steps := []string{"Tags", "Paths", "Import"}
	activeIndex := min(int(m.state), 2)

	var parts []string
	for i, step := range steps {
		num := strconv.Itoa(i + 1)
		switch {
		case i == activeIndex:
			parts = append(parts, activeStepStyle().Render(num+" "+step))
		case i < activeIndex:
			parts = append(parts, successStyle().Render(num+" "+step))
		default:
			parts = append(parts, stepStyle().Render(num+" "+step))
		}
	}

	return strings.Join(parts, stepStyle().Render(" \u2192 "))
}

// tagPreviewView is the tag comparison view, its tags as its list.
func (m *Model) tagPreviewView() fit.View {
	above := m.heading()

	// Until the core has planned the import there is nothing to preview
	if !m.planned() {
		if m.preparing {
			return fit.View{Above: append(above, fit.Lines(
				headerStyle().Render("Preparing import..."),
				"",
				dimStyle().Render("Reading the files' tags, then the release and its cover from MusicBrainz."),
				dimStyle().Render("This may take a moment due to rate limiting."),
			)...)}
		}
		return fit.View{Above: append(above,
			fit.Keep(errorStyle().Render("Cannot import: "+m.err)),
			fit.Line{},
			fit.Keep(dimStyle().Render("[Esc] Close")),
		)}
	}

	// Column widths
	innerWidth := m.innerWidth()
	labelWidth := 14
	valueWidth := (innerWidth - labelWidth - 10) / 2

	// Header row
	header := fmt.Sprintf("%-*s  %-*s  %-*s",
		labelWidth, "Tag",
		valueWidth, "Current",
		valueWidth, "New")

	above = append(above, fit.Lines(
		headerStyle().Render("Tag Changes Preview"),
		"",
		dimStyle().Render(header),
		render.Separator(innerWidth),
	)...)

	// Tag diffs
	items := make([][]string, 0, len(m.tagDiffs))
	for _, diff := range m.tagDiffs {
		oldVal := render.Truncate(diff.OldValue, valueWidth)
		oldVal = render.Pad(oldVal, valueWidth)

		newVal := render.Truncate(diff.NewValue, valueWidth)
		newVal = render.Pad(newVal, valueWidth)

		label := render.Pad(diff.Field, labelWidth)

		oldStyle, newStyle := valueStyle(), valueStyle()
		if diff.Changed {
			oldStyle, newStyle = dimStyle(), changedStyle()
		}
		items = append(items, []string{fmt.Sprintf("%s  %s  %s",
			labelStyle().Render(label),
			oldStyle.Render(oldVal),
			newStyle.Render(newVal))})
	}

	// Cover art status and footer
	below := append(fit.Lines(
		"",
		m.renderCoverArtStatus(),
		"",
		dimStyle().Render(fmt.Sprintf("%d files will be retagged", len(m.plan.Files))),
		"",
	), fit.Keep(dimStyle().Render("[Enter] Continue   [Esc] Cancel")))

	return fit.View{Above: above, Items: items, Below: below, Keys: fit.ScrollKeys}
}

// renderCoverArtStatus renders the cover art status indicator.
func (m *Model) renderCoverArtStatus() string {
	label := labelStyle().Render("Cover Art:    ")

	if len(m.plan.Cover) > 0 {
		size := formatBytes(len(m.plan.Cover))
		return label + successStyle().Render(fmt.Sprintf("%s Found (%s, will be embedded)", completedSymbol, size))
	}

	return label + dimStyle().Render("Not available")
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(b int) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// pathPreviewView is the file path preview, the plan's files as its list.
func (m *Model) pathPreviewView() fit.View {
	innerWidth := m.innerWidth()
	above := m.pathPreviewHeader()
	var items [][]string

	if innerWidth < compactWidthThreshold {
		above = append(above, fit.Lines(render.Separator(innerWidth))...)
		items = m.renderCompactFilePaths(innerWidth)
	} else {
		// Wide layout: single line per file
		numWidth := 3
		arrowWidth := 4 // " → "
		availWidth := innerWidth - numWidth - arrowWidth - 4
		oldWidth := availWidth * 2 / 5
		newWidth := availWidth * 3 / 5

		header := fmt.Sprintf("%*s  %-*s  %-*s",
			numWidth, "#",
			oldWidth, "Current",
			newWidth, "New Path")
		above = append(above, fit.Lines(
			dimStyle().Render(header),
			render.Separator(innerWidth),
		)...)

		for i := range m.plan.Files {
			f := &m.plan.Files[i]

			oldName := render.Truncate(fileName(f.Filename), oldWidth)
			oldName = render.Pad(oldName, oldWidth)

			newPath := render.Truncate(displayNewPath(f), newWidth)
			newPath = render.Pad(newPath, newWidth)
			newStyle := changedStyle()
			if f.Dest == "" {
				newStyle = dimStyle() // not imported
			}

			items = append(items, []string{fmt.Sprintf("%s  %s%s%s",
				dimStyle().Render(trackNum(f, i)),
				valueStyle().Render(oldName),
				dimStyle().Render(sepArrow),
				newStyle.Render(newPath))})
		}
	}

	// Cover art status and help
	below := fit.Lines("", m.renderCoverArtStatus(), "")
	if m.err != "" {
		below = append(below, fit.Keep(errorStyle().Render("Cannot import: "+m.err)), fit.Line{})
	}
	below = append(below, fit.Keep(dimStyle().Render(m.pathPreviewHelpText())))

	return fit.View{Above: above, Items: items, Below: below, Keys: m.scrollHint()}
}

// pathPreviewHeader is what the path preview shows above its file list:
// the title, the library source and the header of the list. The library
// source never gives way.
func (m *Model) pathPreviewHeader() []fit.Line {
	lines := m.heading()

	// Library selector
	switch len(m.librarySources) {
	case 0:
		for _, line := range noLibrarySourceLines() {
			lines = append(lines, fit.Line{Text: line, Kept: line != ""})
		}
	case 1:
		lines = append(lines,
			fit.Keep(headerStyle().Render("Destination: ")+valueStyle().Render(m.librarySources[0])),
			fit.Line{},
		)
	default:
		lines = append(lines, fit.Keep(headerStyle().Render("Destination Library:")))
		for i, source := range m.librarySources {
			if i == m.selectedSource {
				lines = append(lines, fit.Keep(selectedStyle().Render("> "+source)))
			} else {
				lines = append(lines, fit.Keep(dimStyle().Render("  "+source)))
			}
		}
		lines = append(lines, fit.Line{})
	}

	// Path mappings header
	return append(lines, fit.Lines(headerStyle().Render("File Paths"), "")...)
}

// scrollHint names the keys that scroll the path preview. With several
// library sources, j/k select one and do not scroll.
func (m *Model) scrollHint() string {
	if len(m.librarySources) > 1 {
		return "[ctrl+d/u g/G] Scroll"
	}
	return fit.ScrollKeys
}

// fileName is the name of a file of the plan, out of its slskd path.
func fileName(slskdPath string) string {
	return filepath.Base(strings.ReplaceAll(slskdPath, "\\", "/"))
}

// trackNum is the number shown for the plan's file i: the one its name starts
// with, or its place among the files.
func trackNum(f *core.ImportFile, i int) string {
	n := core.ParseTrackNumber(f.Filename)
	if n == 0 {
		n = i + 1
	}
	return fmt.Sprintf("%02d", n)
}

// displayNewPath is where a file goes under the library source, or why it
// goes nowhere.
func displayNewPath(f *core.ImportFile) string {
	if f.Dest == "" {
		return "(no MusicBrainz track: not imported)"
	}
	return f.Dest
}

// renderCompactFilePaths renders the plan's files in compact layout, 3
// lines per file.
// Line 1: track number + original filename
// Lines 2-3: target path wrapped across up to 2 lines.
func (m *Model) renderCompactFilePaths(innerWidth int) [][]string {
	nameWidth := innerWidth - 4 // Leave room for track number
	pathWidth := innerWidth - 6 // Leave room for arrow/continuation indent

	items := make([][]string, 0, len(m.plan.Files))
	for i := range m.plan.Files {
		f := &m.plan.Files[i]

		// Line 1: track number + original filename
		oldName := render.Truncate(fileName(f.Filename), nameWidth)
		line1 := dimStyle().Render(trackNum(f, i)) + "  " + valueStyle().Render(oldName)

		// Lines 2-3: target path (wrapped across up to 2 lines)
		items = append(items, append([]string{line1}, m.renderWrappedPath(displayNewPath(f), pathWidth)...))
	}
	return items
}

// renderWrappedPath renders a path wrapped across up to 2 lines.
func (m *Model) renderWrappedPath(path string, width int) []string {
	arrow := dimStyle().Render("→")
	pathWidth := lipgloss.Width(path)
	if pathWidth <= width {
		// Fits on one line
		line := "    " + arrow + " " + changedStyle().Render(path)
		return []string{line, ""}
	}
	// Wrap to two lines - truncate first line at display width
	line1Path := ansi.Truncate(path, width, "")
	remaining := strings.TrimPrefix(path, line1Path)
	line1 := "    " + arrow + " " + changedStyle().Render(line1Path)
	line2 := "      " + changedStyle().Render(render.Truncate(remaining, width))
	return []string{line1, line2}
}

// pathPreviewHelpText returns the help text for path preview.
func (m *Model) pathPreviewHelpText() string {
	switch len(m.librarySources) {
	case 0:
		return "[Esc] Back"
	case 1:
		return "[Enter] Start Import   [Esc] Back"
	default:
		return "[Enter] Start Import   [j/k] Select Library   [Esc] Back"
	}
}

// noLibrarySourceLines says there is nowhere to import into, and how to add
// a library source: its manager opens from the library view.
func noLibrarySourceLines() []string {
	libraryView := keys.KeysFor(keymap.ActionViewLibrary)[0]
	sourcesManager := keys.KeysFor(keymap.ActionLibrarySources)[0]
	return []string{
		errorStyle().Render("No library source to import into."),
		dimStyle().Render("Press " + libraryView + " for the library view, then " + sourcesManager),
		dimStyle().Render("to open the library sources manager and add a music folder."),
		"",
	}
}

// importingView is the import progress view, the files as its list.
func (m *Model) importingView() fit.View {
	innerWidth := m.innerWidth()
	above := append(m.heading(), fit.Lines(headerStyle().Render("Importing..."), "")...)

	// Progress for each file
	settled := 0
	items := make([][]string, 0, len(m.files))
	for i := range m.files {
		file := &m.files[i]
		var icon, statusText string
		var style lipgloss.Style

		switch file.State {
		case core.FileDone:
			icon = completedSymbol
			statusText = "Done"
			style = successStyle()
		case core.FileAlreadyImported:
			icon = completedSymbol
			statusText = "Already imported"
			style = successStyle()
		case core.FileWorking:
			icon = progressSymbol
			statusText = "Importing..."
			style = changedStyle()
		case core.FileFailed:
			icon = failedSymbol
			statusText = errText(file.Err)
			style = errorStyle()
		case core.FilePending:
			icon = pendingSymbol
			statusText = "Pending"
			style = dimStyle()
		}
		if file.State != core.FilePending && file.State != core.FileWorking {
			settled++
		}

		filename := render.Truncate(filepath.Base(file.Path), innerWidth/2)

		items = append(items, []string{fmt.Sprintf("%s %s  %s",
			style.Render(icon),
			valueStyle().Render(filename),
			style.Render(statusText))})
	}

	below := append(fit.Lines(
		"",
		dimStyle().Render(fmt.Sprintf("Progress: %d/%d files", settled, len(m.files))),
		"",
	), fit.Keep(dimStyle().Render("[Esc] Close (the import continues)")))

	return fit.View{Above: above, Items: items, Below: below}
}

// followedFile is the file the progress list shows: the one being imported,
// else the first still pending, else the last.
func (m *Model) followedFile() int {
	pending := -1
	for i := range m.files {
		switch m.files[i].State { //nolint:exhaustive // the others are settled
		case core.FileWorking:
			return i
		case core.FilePending:
			if pending < 0 {
				pending = i
			}
		}
	}
	if pending >= 0 {
		return pending
	}
	return len(m.files) - 1
}

// errText is why a file failed.
func errText(err error) string {
	if err == nil {
		return "failed"
	}
	return err.Error()
}

// completeView is the completion view, the failed files as its list.
func (m *Model) completeView() fit.View {
	above := m.heading()

	var failed [][]string
	imported, destDir := 0, ""
	for i := range m.files {
		file := &m.files[i]
		switch file.State { //nolint:exhaustive // a finished import leaves a file done or failed
		case core.FileDone, core.FileAlreadyImported:
			imported++
			if destDir == "" {
				destDir = filepath.Dir(file.Dest)
			}
		case core.FileFailed:
			line := fmt.Sprintf("  - %s: %s", filepath.Base(file.Path), errText(file.Err))
			failed = append(failed, []string{errorStyle().Render(line)})
		}
	}

	var below []fit.Line
	if len(failed) == 0 && m.err == "" {
		above = append(above, fit.Lines(
			headerStyle().Render("Import Complete"),
			"",
			successStyle().Render(fmt.Sprintf("%s %d files imported successfully", completedSymbol, imported)),
			successStyle().Render(completedSymbol+" Library index updated"),
		)...)
	} else {
		above = append(above, fit.Lines(
			headerStyle().Render("Import Completed with Errors"),
			"",
		)...)
		if imported > 0 {
			above = append(above, fit.Lines(successStyle().Render(fmt.Sprintf("%s %d files imported successfully", completedSymbol, imported)))...)
		}
		if len(failed) > 0 {
			above = append(above,
				fit.Keep(errorStyle().Render(fmt.Sprintf("%s %d files failed", failedSymbol, len(failed)))),
				fit.Line{},
			)
		}
		if m.err != "" {
			below = append(below, fit.Keep(errorStyle().Render(failedSymbol+" "+m.err)))
		}
	}

	below = append(below, fit.Line{})

	// Destination path
	if destDir != "" {
		below = append(below, fit.Line{Text: dimStyle().Render("Destination: ") + valueStyle().Render(destDir)})
	}

	below = append(below, fit.Line{}, fit.Keep(dimStyle().Render("[Enter] Close")))

	return fit.View{Above: above, Items: failed, Below: below, Keys: fit.ScrollKeys}
}
