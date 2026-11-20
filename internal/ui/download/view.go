package download

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"

	"github.com/llehouerou/waves/internal/ui/cursor"
	"github.com/llehouerou/waves/internal/ui/fit"
	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

const (
	filterOn  = "on"
	filterOff = "off"
)

func selectedStyle() lipgloss.Style {
	return styles.T().BaseStyle().
		Foreground(styles.T().Success).
		Bold(true)
}

func cursorStyle() lipgloss.Style {
	return styles.T().S().Playing
}

func dimStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func errorStyle() lipgloss.Style {
	return styles.T().S().Error
}

func statusStyle() lipgloss.Style {
	return styles.T().S().Warning
}

func typeStyle() lipgloss.Style {
	return styles.T().S().Playing
}

// Step indicator styles
func stepActiveStyle() lipgloss.Style {
	return styles.T().S().Playing
}

func stepCompletedStyle() lipgloss.Style {
	return styles.T().S().Success
}

func stepPendingStyle() lipgloss.Style {
	return styles.T().S().Subtle
}

func stepValueStyle() lipgloss.Style {
	return styles.T().BaseStyle().
		Foreground(styles.T().Success)
}

// View renders the download view.
func (m *Model) View() string {
	return m.padToHeight(m.page().String())
}

// page is what the view of the popup's state shows, in the box of the popup:
// a list shows the row under its cursor, from the row it showed first.
func (m *Model) page() fit.Page {
	v := m.Fit(m.view(), m.Width(), m.Height())
	if c := m.listCursor(); c != nil {
		return v.Following(c.Pos(), c.Offset())
	}
	return v.At(0)
}

// listCursor is the cursor of the list the popup's state shows, nil for a
// state with no list.
func (m *Model) listCursor() *cursor.Cursor {
	switch {
	case m.state.IsReleasesPhase():
		return &m.relCursors[m.relTab]
	case m.state == StateArtistResults:
		return &m.artistCursor
	case m.state.IsReleaseGroupPhase():
		return &m.releaseGroupCursor
	case m.state.IsReleasePhase():
		return &m.releaseCursor
	case m.state.IsSlskdPhase():
		return &m.slskdCursor
	}
	return nil
}

// moveCursor moves the cursor c of the list the popup shows to row to,
// within its n rows, and its page with it: the page the view shows from the
// one it showed, the cursor in view.
func (m *Model) moveCursor(c *cursor.Cursor, to, n int) {
	if n == 0 {
		return
	}
	c.SetPos(min(max(to, 0), n-1))
	c.SetOffset(m.page().First())
}

// view is the view of the popup's state: the new releases, or the step
// indicator and messages above the content of the step, its help line under
// it.
func (m *Model) view() fit.View {
	// Phase 0 owns its whole panel: no step indicator, it is not a step.
	if m.state.IsReleasesPhase() {
		return m.releasesView()
	}

	above := fit.Lines(m.renderStepIndicator(), "")
	if m.errorMsg != "" {
		above = append(above, fit.Keep(errorStyle().Render("Error: "+m.errorMsg)))
	} else {
		above = append(above, fit.Line{})
	}
	status := ""
	if m.statusMsg != "" {
		status = statusStyle().Render(m.statusMsg)
	}
	above = append(above, fit.Line{Text: status})

	var v fit.View
	switch m.state { //nolint:exhaustive // phase 0 returns above
	case StateSearch, StateArtistSearching:
		v = m.searchView()
	case StateArtistResults:
		v = m.artistView()
	case StateReleaseGroupLoading, StateReleaseGroupResults:
		v = m.releaseGroupView()
	case StateReleaseLoading, StateReleaseResults, StateReleaseDetailsLoading:
		v = m.releaseView()
	case StateSlskdSearching, StateSlskdResults, StateDownloading:
		v = m.slskdView()
	}
	v.Above = append(above, v.Above...)
	v.Below = append(v.Below, fit.Keep(dimStyle().Render(m.help())))
	return v
}

// listView is a list of items under its title, a blank line between them
// and under the list, or empty when there are no items.
func listView(title string, items [][]string, empty string) fit.View {
	if len(items) == 0 {
		return fit.View{Above: fit.Lines(dimStyle().Render(empty))}
	}
	return fit.View{
		Above: fit.Lines(dimStyle().Render(title), ""),
		Items: items,
		Below: fit.Lines(""),
	}
}

// textWidth is how many columns a row of the popup's text area has.
func (m *Model) textWidth() int {
	return popup.TextWidth(m.Width())
}

// listRow is a row of a list in width columns, marked when the cursor is on
// it: plain is the row as the cursor shows it, styled as the others.
func listRow(plain, styled string, selected bool, width int) string {
	if selected {
		return render.Cut(cursorStyle().Render("> ")+selectedStyle().Render(plain), width)
	}
	return render.Cut(render.EmptyLine(2)+styled, width)
}

// renderStepIndicator renders the step progress indicator.
func (m *Model) renderStepIndicator() string {
	// Determine current step (1-3)
	currentStep := m.getCurrentStep()

	var b strings.Builder

	// Step 1: Artist
	step1Style := m.getStepStyle(1, currentStep)
	b.WriteString(step1Style.Render("① Artist"))
	if m.selectedArtist != nil {
		b.WriteString(render.EmptyLine(1))
		b.WriteString(stepValueStyle().Render("✓ " + render.Cut(m.selectedArtist.Name, 20)))
	}

	b.WriteString(dimStyle().Render("  →  "))

	// Step 2: Release
	step2Style := m.getStepStyle(2, currentStep)
	b.WriteString(step2Style.Render("② Release"))
	if title, date, ok := m.releaseHeader(); ok {
		info := title
		if date != "" {
			info += " (" + extractYear(date) + ")"
		}
		if m.expectedTracks > 0 {
			info += fmt.Sprintf(" [%d tracks]", m.expectedTracks)
		}
		b.WriteString(render.EmptyLine(1))
		b.WriteString(stepValueStyle().Render("✓ " + render.Cut(info, 40)))
	}

	b.WriteString(dimStyle().Render("  →  "))

	// Step 3: Source
	step3Style := m.getStepStyle(3, currentStep)
	b.WriteString(step3Style.Render("③ Source"))

	return b.String()
}

// releaseHeader is the release the header shows: the selected release group,
// or the row of the new releases the flow was entered with until the release
// details bring the group.
func (m *Model) releaseHeader() (title, date string, ok bool) {
	switch {
	case m.selectedReleaseGroup != nil:
		return m.selectedReleaseGroup.Title, m.selectedReleaseGroup.FirstRelease, true
	case m.relEntered != nil:
		return m.relEntered.ReleaseName, m.relEntered.ReleaseDate, true
	}
	return "", "", false
}

// getCurrentStep returns the current step number (1-3).
func (m *Model) getCurrentStep() int {
	switch m.state { //nolint:exhaustive // phase 0 has no step: View returns before
	case StateSearch, StateArtistSearching, StateArtistResults:
		return 1
	case StateReleaseGroupLoading, StateReleaseGroupResults, StateReleaseLoading, StateReleaseResults, StateReleaseDetailsLoading:
		return 2
	case StateSlskdSearching, StateSlskdResults, StateDownloading:
		return 3
	}
	return 1
}

// getStepStyle returns the appropriate style for a step.
func (m *Model) getStepStyle(step, currentStep int) lipgloss.Style {
	if step == currentStep {
		return stepActiveStyle()
	}
	if step < currentStep {
		return stepCompletedStyle()
	}
	return stepPendingStyle()
}

// help is the help line of the popup's state, the widest that fits its
// text area.
func (m *Model) help() string {
	width := m.textWidth()
	switch m.state {
	case StateReleasesLoading, StateReleasesResults:
		return m.releasesHelp(m.releasesWidth())
	case StateSearch:
		return "Enter: Search | Esc: Close"
	case StateArtistSearching:
		return "Searching artists... | Esc: Close"
	case StateArtistResults:
		return fit.Widest(width,
			"↑/↓: Navigate | Enter: Select | Backspace: Back | Esc: Close",
			"↑/↓: Move | Enter: Select | Backspace: Back | Esc: Close",
			"↵ select · ⌫ back · esc",
		)
	case StateReleaseGroupLoading:
		return "Loading releases... | Esc: Close"
	case StateReleaseGroupResults:
		albums := filterOn
		if !m.albumsOnly {
			albums = filterOff
		}
		return fit.Widest(width,
			"↑/↓: Navigate | Enter: Select | a: Albums ["+albums+"] | Backspace: Back | Esc: Close",
			"Enter: Select | a: Albums ["+albums+"] | Bksp: Back | Esc: Close",
			"↵ select · a: Albums ["+albums+"] · esc",
		)
	case StateReleaseLoading:
		return "Loading track info... | Esc: Close"
	case StateReleaseResults:
		dedup := filterOn
		if !m.deduplicateRelease {
			dedup = filterOff
		}
		return fit.Widest(width,
			"↑/↓: Navigate | Enter: Select | d: Dedup ["+dedup+"] | Backspace: Back | Esc: Close",
			"Enter: Select | d: Dedup ["+dedup+"] | Bksp: Back | Esc: Close",
			"↵ select · d: Dedup ["+dedup+"] · esc",
		)
	case StateReleaseDetailsLoading:
		return "Loading release details... | Esc: Close"
	case StateSlskdSearching:
		return "Searching slskd... | Esc: Close"
	case StateSlskdResults:
		return fit.Widest(width,
			"↑/↓: Navigate | Enter: Download | f/s/t: Filters | Backspace: Back | Esc: Close",
			"↵: Download | f/s/t: Filters | Bksp: Back | Esc: Close",
			"↵ download · f/s/t filters · esc",
		)
	case StateDownloading:
		return "Backspace: Back | Esc: Close"
	}
	return ""
}

// formatSize formats a file size in human-readable form.
// Uses binary calculation (1024) with SI notation (KB, MB, GB).
func formatSize(bytes int64) string {
	if bytes < 0 {
		bytes = 0
	}
	s := humanize.IBytes(uint64(bytes)) //nolint:gosec // bytes is guaranteed non-negative above
	// Convert IEC notation to SI: GiB→GB, MiB→MB, KiB→KB
	s = strings.ReplaceAll(s, "iB", "B")
	return s
}

// padToHeight pads the content to the rows of the popup's own box, to
// prevent layout jitter: a view that fits draws a box of the popup's height.
func (m *Model) padToHeight(content string) string {
	rows := 0
	for line := range strings.SplitSeq(content, "\n") {
		rows += fit.Rows(line, m.textWidth())
	}
	if padding := popup.TextRows(m.Height()) - rows; padding > 0 {
		content += strings.Repeat("\n", padding)
	}
	return content
}
