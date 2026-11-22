package retag

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/fit"
	"github.com/llehouerou/waves/internal/ui/mbworkflow"
	"github.com/llehouerou/waves/internal/ui/popup"
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

func titleStyle() lipgloss.Style {
	return styles.T().S().Title
}

func headerStyle() lipgloss.Style {
	return styles.T().S().Muted
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

func cursorStyle() lipgloss.Style {
	return styles.T().S().Playing
}

func typeStyle() lipgloss.Style {
	return styles.T().S().Playing
}

// View renders the retag popup.
func (m *Model) View() string {
	if m.Width() == 0 || m.Height() == 0 {
		return ""
	}
	return m.page().String()
}

// page is what the view of the popup's state shows, in the box of the
// popup: a list the user moves in shows its cursor, the tags and the failed
// files are scrolled where the user left them, and the progress list follows
// the retag.
func (m *Model) page() fit.Page {
	switch m.state {
	case StateLoading:
		return m.boxed(m.loadingView("Reading tags from files...")).At(0)
	case StateSearching:
		return m.boxed(m.loadingView(m.statusMsg)).At(0)
	case StateReleaseGroupResults:
		c := m.releaseGroupCursor
		return m.boxed(m.releaseGroupsView()).Following(c.Pos(), c.Offset())
	case StateReleaseLoading:
		return m.boxed(m.loadingView("Loading releases...")).At(0)
	case StateReleaseResults:
		c := m.releaseCursor
		return m.boxed(m.releasesView()).Following(c.Pos(), c.Offset())
	case StateReleaseDetailsLoading:
		return m.boxed(m.loadingView("Loading release details...")).At(0)
	case StateTagPreview:
		return m.boxed(m.tagPreviewView()).At(m.tagOffset)
	case StateRetagging:
		return m.boxed(m.retaggingView()).Showing(m.followedFile())
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

// textWidth is how many columns a row of the popup's text area has: no line
// wider takes a single row. The rows of a list are cut to it.
func (m *Model) textWidth() int {
	return popup.TextWidth(m.Width())
}

// title is the line each view starts with: the album retagged.
func (m *Model) title() string {
	return titleStyle().Render(fmt.Sprintf("Retag: %s - %s", m.albumArtist, m.albumName))
}

// heading is what the views with nothing to say of the release show first:
// the title and a separator.
func (m *Model) heading() []fit.Line {
	return fit.Lines(m.title(), "", render.Separator(m.innerWidth()), "")
}

// innerWidth is the width of the separators, of the file names and of the
// tag preview's columns: two columns short of the text width, the margin they
// have always kept.
func (m *Model) innerWidth() int {
	return m.Width() - 8
}

// renderSearchMethodInfo renders information about how the search was performed.
func (m *Model) renderSearchMethodInfo() []string {
	var lines []string

	// Show which MusicBrainz IDs were found in tags
	if m.foundMBReleaseID != "" {
		lines = append(lines, dimStyle().Render("Release ID: ")+valueStyle().Render(m.foundMBReleaseID))
	}
	if m.foundMBReleaseGroupID != "" {
		lines = append(lines, dimStyle().Render("Release Group ID: ")+valueStyle().Render(m.foundMBReleaseGroupID))
	}
	if m.foundMBArtistID != "" {
		lines = append(lines, dimStyle().Render("Artist ID: ")+valueStyle().Render(m.foundMBArtistID))
	}

	// Show search method
	if m.searchMethod != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, dimStyle().Render("Method: ")+headerStyle().Render(m.searchMethod))
	}

	if len(lines) > 0 {
		lines = append(lines, "")
	}

	return lines
}

// loadingView is a view that waits for message, or tells what failed.
func (m *Model) loadingView(message string) fit.View {
	above := m.heading()
	if m.errorMsg != "" {
		return fit.View{Above: append(above,
			fit.Keep(errorStyle().Render("Error: "+m.errorMsg)),
			fit.Line{},
			fit.Keep(dimStyle().Render("[Esc] Close")),
		)}
	}
	return fit.View{Above: append(append(above, fit.Lines(headerStyle().Render(message), "")...),
		fit.Keep(dimStyle().Render("[Esc] Cancel")),
	)}
}

// releaseGroupsView is the release group selection view, the groups as its
// list.
func (m *Model) releaseGroupsView() fit.View {
	above := fit.Lines(m.title(), "")
	above = append(above, fit.Lines(m.renderSearchMethodInfo()...)...)
	above = append(above, fit.Lines(render.Separator(m.innerWidth()), "")...)
	above = withCut(above, m.foundGroupsCut, "release groups")

	// Search input if in search mode
	if m.searchMode {
		above = append(above, fit.Keep(headerStyle().Render("Search: ")+m.searchInput.View()), fit.Line{})
	}
	if m.errorMsg != "" {
		above = append(above, fit.Keep(errorStyle().Render("Error: "+m.errorMsg)), fit.Line{})
	}

	width := m.textWidth()
	if len(m.releaseGroups) == 0 {
		empty := "No release groups found"
		switch {
		case len(m.foundGroups) == 1:
			empty = "1 release group hidden by the albums-only filter"
		case len(m.foundGroups) > 1:
			empty = fmt.Sprintf("%d release groups hidden by the albums-only filter", len(m.foundGroups))
		}
		above = append(above, fit.Lines(dimStyle().Render(empty), "")...)
		return fit.View{Above: above, Below: []fit.Line{fit.Keep(dimStyle().Render(m.releaseGroupHelp(width, false)))}}
	}

	above = append(above, fit.Lines(headerStyle().Render("Select a release group:"), "")...)
	items := make([][]string, len(m.releaseGroups))
	for i := range m.releaseGroups {
		items[i] = []string{listRow(m.formatReleaseGroup(&m.releaseGroups[i], width-2), i == m.releaseGroupCursor.Pos())}
	}
	below := append(fit.Lines(""), fit.Keep(dimStyle().Render(m.releaseGroupHelp(width, true))))
	return fit.View{Above: above, Items: items, Below: below}
}

// withCut is above followed by the line that tells that MusicBrainz has more
// of what the list shows than were read, when cut says so: a line that never
// gives way.
func withCut(above []fit.Line, cut mbworkflow.Cut, what string) []fit.Line {
	if notice := cut.Notice(what); notice != "" {
		above = append(above, fit.Keep(headerStyle().Render(notice)), fit.Line{})
	}
	return above
}

// listRow is a row of a list, marked when the cursor is on it: line is
// already cut to the text width, less the two columns of the mark.
func listRow(line string, selected bool) string {
	if selected {
		return cursorStyle().Render("> ") + selectedStyle().Render(line)
	}
	return "  " + line
}

// releaseGroupHelp is the help line of the list of release groups, the
// widest that fits width columns: with selecting, it names the key that
// selects a row first. It names the key of the albums-only filter and its
// state.
func (m *Model) releaseGroupHelp(width int, selecting bool) string {
	filter := "off"
	if m.albumsOnly {
		filter = "on"
	}
	if !selecting {
		return "[/] Search   [a] Albums only: " + filter + "   [Esc] Close"
	}
	return fit.Widest(width,
		"[Enter] Select   [/] Search   [a] Albums only: "+filter+"   [Esc] Close",
		"[↵] Select  [/] Search  [a] Albums only: "+filter+"  [Esc] Close",
		"↵ select · / search · a albums only: "+filter+" · esc",
	)
}

// formatReleaseGroup formats a single release group in width columns: its
// title gives way to what follows it.
func (m *Model) formatReleaseGroup(rg *core.MBReleaseGroup, width int) string {
	// Always show artist first for clarity
	var head string
	if rg.Artist != "" {
		if strings.EqualFold(rg.Artist, m.albumArtist) {
			// Matching artist - show in green
			head = successStyle().Render(rg.Artist) + " - "
		} else {
			// Different artist - show dimmed
			head = dimStyle().Render(rg.Artist) + " - "
		}
	}

	var tail []string
	if rg.FirstRelease != "" {
		year := rg.FirstRelease
		if len(year) > 4 {
			year = year[:4]
		}
		tail = append(tail, fmt.Sprintf(" (%s)", year))
	}

	if rg.PrimaryType != "" {
		tail = append(tail, " "+typeStyle().Render(fmt.Sprintf("[%s]", rg.PrimaryType)))
	}

	if len(rg.SecondaryTypes) > 0 {
		tail = append(tail, " "+dimStyle().Render("+"+strings.Join(rg.SecondaryTypes, "+")))
	}

	return cutTitle(head, rg.Title, strings.Join(tail, ""), width)
}

// cutTitle is head, title and tail on one row of width columns: the title
// gives way first, then the row is cut.
func cutTitle(head, title, tail string, width int) string {
	room := width - lipgloss.Width(head) - lipgloss.Width(tail)
	if lipgloss.Width(title) > room {
		title = render.Cut(title, max(room, 0))
	}
	return render.Cut(head+title+tail, width)
}

// releasesView is the release selection view, the releases as its list.
func (m *Model) releasesView() fit.View {
	above := fit.Lines(m.title(), "")

	// Show selected release group
	if m.listedGroupTitle != "" {
		above = append(above, fit.Lines(dimStyle().Render("Release Group: ")+valueStyle().Render(m.listedGroupTitle))...)
	}
	above = append(above, fit.Lines(render.Separator(m.innerWidth()), "")...)
	above = withCut(above, m.releasesCut, "releases")

	if m.errorMsg != "" {
		above = append(above, fit.Keep(errorStyle().Render("Error: "+m.errorMsg)), fit.Line{})
	}

	if len(m.releases) == 0 {
		above = append(above, fit.Lines(dimStyle().Render("No releases found"), "")...)
		return fit.View{Above: above, Below: []fit.Line{fit.Keep(dimStyle().Render("[Backspace] Back   [Esc] Close"))}}
	}

	above = append(above, fit.Lines(headerStyle().Render("Select a release:"), "")...)
	items := make([][]string, len(m.releases))
	for i := range m.releases {
		items[i] = []string{listRow(m.formatRelease(&m.releases[i], m.textWidth()-2), i == m.releaseCursor.Pos())}
	}
	below := append(fit.Lines(""), fit.Keep(dimStyle().Render("[Enter] Select   [Backspace] Back   [Esc] Close")))
	return fit.View{Above: above, Items: items, Below: below}
}

// formatRelease formats a single release in width columns: its title gives
// way to what follows it.
func (m *Model) formatRelease(r *core.MBRelease, width int) string {
	var parts []string

	localTrackCount := len(m.currentTags)
	if r.TrackCount > 0 {
		if r.TrackCount == localTrackCount {
			// Track count matches - show in green with checkmark
			parts = append(parts, successStyle().Render(fmt.Sprintf("[%d tracks %s]", r.TrackCount, completedSymbol)))
		} else {
			// Track count doesn't match
			parts = append(parts, typeStyle().Render(fmt.Sprintf("[%d tracks]", r.TrackCount)))
		}
	}

	if r.Date != "" {
		year := r.Date
		if len(year) > 4 {
			year = year[:4]
		}
		parts = append(parts, fmt.Sprintf("(%s)", year))
	}

	if r.Country != "" {
		parts = append(parts, dimStyle().Render(fmt.Sprintf("[%s]", r.Country)))
	}

	if r.Formats != "" {
		parts = append(parts, dimStyle().Render(r.Formats))
	}

	var tail string
	if len(parts) > 0 {
		tail = " " + strings.Join(parts, " ")
	}
	return cutTitle("", r.Title, tail, width)
}

// tagPreviewView is the tag comparison view, its tags as its list.
func (m *Model) tagPreviewView() fit.View {
	innerWidth := m.innerWidth()

	// Column widths
	labelWidth := 14
	valueWidth := (innerWidth - labelWidth - 10) / 2

	// Header row
	header := fmt.Sprintf("%-*s  %-*s  %-*s",
		labelWidth, "Tag",
		valueWidth, "Current",
		valueWidth, "New")

	above := fit.Lines(m.title(), "")

	// Show selected release info
	if m.releaseDetails != nil {
		above = append(above, fit.Lines(
			dimStyle().Render("Release: ")+valueStyle().Render(m.releaseDetails.Title),
		)...)
		if m.releaseDetails.Date != "" {
			above = append(above, fit.Lines(
				dimStyle().Render("Date: ")+valueStyle().Render(m.releaseDetails.Date),
			)...)
		}
	}

	above = append(above, fit.Lines(
		render.Separator(innerWidth),
		"",
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
	var helpText string
	if !m.coverArtFetched {
		helpText = "Waiting for cover art...   [Backspace] Back   [Esc] Cancel"
	} else {
		helpText = "[Enter] Retag   [Backspace] Back   [Esc] Cancel"
	}

	below := fit.Lines(
		"",
		m.renderCoverArtStatus(),
		"",
		dimStyle().Render(fmt.Sprintf("%d files will be retagged", len(m.currentTags))),
		"",
	)
	if m.errorMsg != "" {
		below = append(below, fit.Keep(errorStyle().Render("Error: "+m.errorMsg)), fit.Line{})
	}
	below = append(below, fit.Keep(dimStyle().Render(helpText)))

	return fit.View{Above: above, Items: items, Below: below, Keys: fit.ScrollKeys}
}

// renderCoverArtStatus renders the cover art status indicator.
func (m *Model) renderCoverArtStatus() string {
	label := labelStyle().Render("Cover Art:    ")

	if !m.coverArtFetched {
		return label + dimStyle().Render("Fetching...")
	}

	if len(m.coverArt) > 0 {
		size := formatBytes(len(m.coverArt))
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

// retaggingView is the retag progress view, the files as its list.
func (m *Model) retaggingView() fit.View {
	above := append(m.heading(), fit.Lines(headerStyle().Render("Retagging..."), "")...)
	if m.statusMsg != "" {
		above = append(above, fit.Lines(dimStyle().Render(m.statusMsg), "")...)
	}

	// Progress for each file
	settled := 0
	items := make([][]string, 0, len(m.files))
	for i := range m.files {
		file := &m.files[i]
		var icon, statusText string
		var style lipgloss.Style

		switch file.State { //nolint:exhaustive // a retag moves no file: none is already imported
		case core.FileDone:
			icon = completedSymbol
			statusText = "Done"
			style = successStyle()
			settled++
		case core.FileWorking:
			icon = progressSymbol
			statusText = "Retagging..."
			style = changedStyle()
		case core.FileFailed:
			icon = failedSymbol
			statusText = errText(file.Err)
			style = errorStyle()
			settled++
		default:
			icon = pendingSymbol
			statusText = "Pending"
			style = dimStyle()
		}

		filename := render.Truncate(filepath.Base(file.Path), m.innerWidth()/2)
		items = append(items, []string{fmt.Sprintf("%s %s  %s",
			style.Render(icon),
			valueStyle().Render(filename),
			style.Render(statusText))})
	}

	below := append(fit.Lines(
		"",
		dimStyle().Render(fmt.Sprintf("Progress: %d/%d files", settled, len(m.files))),
		"",
	), fit.Keep(dimStyle().Render("[Esc] Close (retag continues in background)")))

	return fit.View{Above: above, Items: items, Below: below}
}

// followedFile is the file the progress list shows: the one being retagged,
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
	retagged := 0
	for i := range m.files {
		file := &m.files[i]
		switch file.State { //nolint:exhaustive // a finished retag leaves a file done or failed
		case core.FileDone:
			retagged++
		case core.FileFailed:
			line := fmt.Sprintf("  - %s: %s", filepath.Base(file.Path), errText(file.Err))
			failed = append(failed, []string{errorStyle().Render(line)})
		}
	}

	if len(failed) == 0 && m.errorMsg == "" {
		above = append(above, fit.Lines(
			headerStyle().Render("Retag Complete"),
			"",
			successStyle().Render(fmt.Sprintf("%s %d files retagged successfully", completedSymbol, retagged)),
			successStyle().Render(completedSymbol+" Library index updated"),
		)...)
	} else {
		above = append(above, fit.Lines(
			headerStyle().Render("Retag Completed with Errors"),
			"",
		)...)
		if retagged > 0 {
			above = append(above, fit.Lines(successStyle().Render(fmt.Sprintf("%s %d files retagged successfully", completedSymbol, retagged)))...)
		}
		if len(failed) > 0 {
			above = append(above,
				fit.Keep(errorStyle().Render(fmt.Sprintf("%s %d files failed", failedSymbol, len(failed)))),
				fit.Line{},
			)
		}
	}

	var below []fit.Line
	if m.errorMsg != "" {
		below = append(below, fit.Line{}, fit.Keep(errorStyle().Render(m.errorMsg)))
	}
	below = append(below, fit.Line{}, fit.Keep(dimStyle().Render("[Enter] Close")))

	return fit.View{Above: above, Items: failed, Below: below, Keys: fit.ScrollKeys}
}
