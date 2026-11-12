package download

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/icons"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// maxSeedsShown is how many recommending seeds a discovery row names.
const maxSeedsShown = 2

// markerWidth fits the widest icon style ("[~]" in the "none" style).
const markerWidth = 3

// minTextWidth is the space "Artist — Title" keeps before the meta gives way.
const minTextWidth = 24

// maxQueryShown caps the search query drawn in the header.
const maxQueryShown = 20

// renderReleases renders the new releases list: header, discreet notices, and a
// date-grouped body budgeted in rendered lines. Every line is cut to the panel
// width: one release is one line, never a wrapped one.
func (m *Model) renderReleases() string {
	width := m.releasesWidth()

	var b strings.Builder
	b.WriteString(m.renderReleasesHeader(width))
	b.WriteString("\n")

	for _, notice := range m.releasesNotices(width) {
		b.WriteString(notice)
		b.WriteString("\n")
	}

	rows := m.visibleReleases()
	if len(rows) == 0 {
		if m.relRefreshing || m.state == StateReleasesLoading {
			b.WriteString(dimStyle().Render("Loading releases…"))
		} else {
			b.WriteString(dimStyle().Render("No releases"))
		}
		return b.String()
	}

	b.WriteString("\n")
	b.WriteString(m.renderReleaseRows(rows, width))
	return b.String()
}

// releasesWidth is the width a rendered line may occupy. The popup model is
// sized with the box width, while popup.RenderBordered spends 6 columns on the
// border and its padding: budgeting the full width is what wraps the rows.
func (m *Model) releasesWidth() int {
	return max(m.Width()-6, 20)
}

// renderReleasesHeader renders the tabs with their counts plus, when they fit,
// the filter, the search query and the title. Nothing ever wraps.
func (m *Model) renderReleasesHeader(width int) string {
	s := styles.T().S()
	counts := m.releasesTabCounts()

	tabs := [2]string{
		fmt.Sprintf(" %s %d ", relTabRecent, counts[relTabRecent]),
		fmt.Sprintf(" %s %d ", relTabUpcoming, counts[relTabUpcoming]),
	}
	title := "New releases"
	filter := "filter: " + m.relFilter.String()
	query := m.releasesQueryLabel()

	// Tabs and query always stay; the filter, then the title, give way.
	room := width - lipgloss.Width(tabs[0]) - lipgloss.Width(tabs[1])
	if query != "" {
		room -= 3 + lipgloss.Width(query)
	}
	showFilter := room >= 3+lipgloss.Width(filter)
	if showFilter {
		room -= 3 + lipgloss.Width(filter)
	}
	showTitle := room >= 3+lipgloss.Width(title)

	var head strings.Builder
	if showTitle {
		head.WriteString(s.Title.Render(title) + "   ")
	}
	for i, tab := range tabs {
		if relTab(i) == m.relTab {
			head.WriteString(s.Cursor.Bold(true).Render(tab))
		} else {
			head.WriteString(s.Subtle.Render(tab))
		}
	}
	if showFilter {
		head.WriteString("   " + s.Muted.Render(filter))
	}
	if query != "" {
		if m.relSearching {
			head.WriteString("   " + s.Cursor.Render(query))
		} else {
			head.WriteString("   " + s.Muted.Render(query))
		}
	}
	return head.String()
}

// releasesHelp returns the widest help line that fits, cut as a last resort.
func (m *Model) releasesHelp(width int) string {
	var variants []string
	switch {
	case m.state == StateReleasesLoading:
		variants = []string{"Loading releases... | Esc: Close", "Loading…"}
	case m.relSearching:
		variants = []string{"Type to search | Enter: Keep | Esc: Clear", "Enter: Keep | Esc: Clear"}
	default:
		variants = []string{
			"↑/↓: Move | Enter: Download | Tab: Tabs | f: Filter | /: Search | r: Refresh | Backspace: Close",
			"Enter: Download | Tab: Tabs | f: Filter | /: Search | r: Refresh",
			"↵ download · tab · f · / · r · esc",
		}
	}

	help := variants[len(variants)-1]
	for _, variant := range variants {
		if lipgloss.Width(variant) <= width {
			help = variant
			break
		}
	}
	return render.TruncateEllipsis(help, width)
}

// releasesQueryLabel is the search query as shown in the header.
func (m *Model) releasesQueryLabel() string {
	if !m.relSearching && m.relQuery == "" {
		return ""
	}
	query := m.relQuery
	if runes := []rune(query); len(runes) > maxQueryShown {
		query = "…" + string(runes[len(runes)-maxQueryShown:])
	}
	if m.relSearching {
		return "/" + query + "█"
	}
	return "/" + query
}

// releasesTabCounts counts the filtered rows of each tab.
func (m *Model) releasesTabCounts() [2]int {
	today := time.Now().Format(time.DateOnly)
	var counts [2]int
	for i := range m.relRows {
		r := &m.relRows[i]
		if !m.matchesFilter(r) {
			continue
		}
		if r.ReleaseDate > today {
			counts[relTabUpcoming]++
		} else {
			counts[relTabRecent]++
		}
	}
	return counts
}

// releasesNotices returns the discreet lines shown above the list.
func (m *Model) releasesNotices(width int) []string {
	var notices []string
	add := func(text string, style lipgloss.Style) {
		if text != "" {
			notices = append(notices, style.Render(render.TruncateEllipsis(text, width)))
		}
	}
	add(m.relErr, errorStyle())
	if !m.relDiscoveries {
		add("Last.fm API key needed for discoveries", dimStyle())
	}
	if m.relRefreshing && len(m.relRows) > 0 {
		add("Refreshing releases…", dimStyle())
	}
	add(m.errorMsg, errorStyle())
	add(m.statusMsg, statusStyle())
	return notices
}

// renderReleaseRows renders date groups until the line budget is spent.
func (m *Model) renderReleaseRows(rows []ReleaseRow, width int) string {
	budget := m.releasesBodyHeight()
	cur := m.relCursors[m.relTab]
	pos := min(cur.Pos(), len(rows)-1)
	start := releasesStart(rows, min(cur.Offset(), pos), pos, budget)

	var b strings.Builder
	lines, lastDate := 0, ""
	for i := start; i < len(rows); i++ {
		need := 1
		if rows[i].ReleaseDate != lastDate {
			need = 2
		}
		if lines+need > budget {
			break
		}
		if rows[i].ReleaseDate != lastDate {
			lastDate = rows[i].ReleaseDate
			b.WriteString(statusStyle().Render(dateHeader(lastDate, width)))
			b.WriteString("\n")
		}
		queued := rows[i].Downloading || m.relQueued[rows[i].ReleaseGroupMBID]
		b.WriteString(renderReleaseRow(rows[i], width, i == pos, queued))
		b.WriteString("\n")
		lines += need
	}
	return b.String()
}

// releasesStart returns the first row to render so the cursor row stays within
// the line budget, date headers included.
func releasesStart(rows []ReleaseRow, offset, pos, budget int) int {
	start := max(offset, 0)
	for start < pos && renderedLines(rows, start, pos) > budget {
		start++
	}
	return start
}

// renderedLines counts the lines rows[from:to] take, date headers included.
func renderedLines(rows []ReleaseRow, from, to int) int {
	lines, lastDate := 0, ""
	for i := from; i <= to && i < len(rows); i++ {
		lines++
		if rows[i].ReleaseDate != lastDate {
			lines++
			lastDate = rows[i].ReleaseDate
		}
	}
	return lines
}

// renderReleaseRow renders "marker  Artist — Title" with discreet meta pushed
// right. No aligned columns: that is what survives narrow widths and CJK.
// The title keeps minTextWidth columns; the meta gives up its seeds, then the
// seed count, rather than eating into it.
func renderReleaseRow(r ReleaseRow, width int, selected, queued bool) string {
	s := styles.T().S()
	text := r.ArtistCreditName + " — " + r.ReleaseName
	// cursor prefix + marker + space + text + two-space gap + meta == width.
	avail := width - 2 - markerWidth - 3
	minText := min(minTextWidth, avail)

	meta := ""
	for _, candidate := range metaVariants(r) {
		meta = candidate
		if avail-lipgloss.Width(meta) >= minText {
			break
		}
	}
	// Even the shortest meta yields rather than push the row past the width.
	meta = render.TruncateEllipsis(meta, max(avail-minText, 0))

	line := releaseMarker(r, queued) + " " +
		render.TruncateAndPadEllipsis(text, avail-lipgloss.Width(meta)) + "  " +
		s.Subtle.Render(meta)

	switch {
	case selected:
		return s.Cursor.Render("> " + line)
	case r.Owned, queued:
		// Owned or already downloading: same muted row, nothing left to do here.
		return s.Subtle.Render("  " + line)
	default:
		return s.Base.Render("  " + line)
	}
}

// releaseMarker shows library vs discovery, already-owned and queued releases.
func releaseMarker(r ReleaseRow, queued bool) string {
	switch {
	case queued:
		return padMarker(icons.Queued())
	case r.Owned:
		return padMarker(icons.InLibrary())
	case !r.InLibrary:
		return padMarker(icons.Radio())
	default:
		return padMarker(icons.Artist())
	}
}

func padMarker(s string) string {
	return render.TruncateAndPadEllipsis(strings.TrimSpace(s), markerWidth)
}

// metaVariants lists the right-hand meta from the most to the least verbose:
// type + seed names, type + seed count, then the type alone.
func metaVariants(r ReleaseRow) []string {
	if r.InLibrary || len(r.Seeds) == 0 {
		return []string{r.PrimaryType}
	}
	seeds := r.Seeds
	if len(seeds) > maxSeedsShown {
		seeds = seeds[:maxSeedsShown]
	}
	names := "← " + strings.Join(seeds, ", ")
	if len(r.Seeds) > maxSeedsShown {
		names += fmt.Sprintf(" +%d", len(r.Seeds)-maxSeedsShown)
	}
	return []string{
		r.PrimaryType + " · " + names,
		fmt.Sprintf("%s · ←%d", r.PrimaryType, len(r.Seeds)),
		r.PrimaryType,
	}
}

// dateHeader renders "── Today ─────────" for a release date.
func dateHeader(date string, width int) string {
	label := " " + humanDate(date) + " "
	return "──" + label + strings.Repeat("─", max(width-lipgloss.Width(label)-2, 0))
}

// humanDate formats a release date as Today, Yesterday, Tomorrow or "Mon 2 Jan 2006".
func humanDate(date string) string {
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, t.Location())
	switch int(t.Sub(today).Hours() / 24) {
	case 0:
		return "Today"
	case 1:
		return "Tomorrow"
	case -1:
		return "Yesterday"
	}
	return t.Format("Mon 2 Jan 2006")
}
