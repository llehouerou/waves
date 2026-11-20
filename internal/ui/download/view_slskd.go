package download

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/fit"
	"github.com/llehouerou/waves/internal/ui/render"
)

// slskdView is the slskd search results as a table, the results as its
// list, the filters under it.
func (m *Model) slskdView() fit.View {
	if len(m.slskdResults) == 0 {
		// Don't show "no sources" while still searching
		if m.state == StateSlskdSearching {
			return fit.View{Above: fit.Lines("")}
		}
		return fit.View{Above: fit.Lines(dimStyle().Render("No sources found"))}
	}

	width := m.textWidth()
	columns := m.slskdColumns()
	header := render.Cut("  "+slskdRow(columns, func(c slskdColumn) string { return c.title }), width)
	above := fit.Lines(
		dimStyle().Render("Select a download source:"),
		"",
		dimStyle().Render(header),
		render.Separator(lipgloss.Width(header)),
	)

	cursorPos := m.slskdCursor.Pos()
	items := make([][]string, len(m.slskdResults))
	for i := range m.slskdResults {
		r := &m.slskdResults[i]
		row := slskdRow(columns, func(c slskdColumn) string { return c.value(r) })
		items[i] = []string{listRow(row, dimStyle().Render(row), i == cursorPos, width)}
	}

	below := fit.Lines("", m.renderFilterControls(), m.renderFilterStats())
	return fit.View{Above: above, Items: items, Below: below}
}

// slskdColumn is a column of the slskd results table: the header and the
// rows are laid out from the same columns.
type slskdColumn struct {
	title string
	width int
	// right aligns the column to the right. Only for ASCII values: they are
	// padded by fmt, which counts code points, not display columns.
	right bool
	// value is the text of a result in the column, at most width columns wide.
	value func(r *core.SlskdResult) string
}

// slskdColumns returns the columns of the slskd results table, laid out
// from the text area: the columns of fixed width keep it, the User column,
// and the Directory column of the wide layout, take what they leave.
func (m *Model) slskdColumns() []slskdColumn {
	const (
		colFormat   = 8
		colBitRate  = 6
		colFiles    = 5
		colSize     = 9
		colSpeed    = 10
		minUser     = 4
		maxUser     = 18
		minDirWidth = 20
		maxDirWidth = 50
		// the cursor marker, the fixed columns and the space before each
		fixedWidth = 2 + colFormat + colBitRate + colFiles + colSize + colSpeed + 5
	)
	room := m.textWidth() - fixedWidth
	colDir := 0
	if !m.isCompactMode() {
		colDir = min(max(room-1-maxUser, minDirWidth), maxDirWidth)
		room -= colDir + 1
	}
	colUser := min(max(room, minUser), maxUser)

	columns := []slskdColumn{{title: "User", width: colUser, value: func(r *core.SlskdResult) string {
		return render.Cut(r.Username, colUser)
	}}}
	if colDir > 0 {
		columns = append(columns, slskdColumn{title: "Directory", width: colDir, value: func(r *core.SlskdResult) string {
			return render.CutLeft(r.Directory, colDir)
		}})
	}
	return append(columns,
		slskdColumn{title: "Format", width: colFormat, value: func(r *core.SlskdResult) string {
			return r.Format
		}},
		slskdColumn{title: "kbps", width: colBitRate, right: true, value: func(r *core.SlskdResult) string {
			return formatBitRate(r.BitRate)
		}},
		slskdColumn{title: "Files", width: colFiles, right: true, value: func(r *core.SlskdResult) string {
			return strconv.Itoa(r.FileCount)
		}},
		slskdColumn{title: "Size", width: colSize, right: true, value: func(r *core.SlskdResult) string {
			return formatSize(r.TotalSize)
		}},
		slskdColumn{title: "Speed", width: colSpeed, right: true, value: func(r *core.SlskdResult) string {
			return formatSpeed(r.UploadSpeed)
		}},
	)
}

// slskdRow lays out the cells that cell returns for columns, separated by a
// space, each padded to the width of its column by display width.
func slskdRow(columns []slskdColumn, cell func(c slskdColumn) string) string {
	parts := make([]string, len(columns))
	for i, c := range columns {
		if c.right {
			parts[i] = fmt.Sprintf("%*s", c.width, cell(c))
		} else {
			parts[i] = render.Pad(cell(c), c.width)
		}
	}
	return strings.Join(parts, " ")
}

// renderFilterControls renders the current filter settings.
func (m *Model) renderFilterControls() string {
	parts := make([]string, 0, 3)

	// Format filter
	var formatLabel string
	switch m.formatFilter {
	case core.FormatBoth:
		formatLabel = "Both"
	case core.FormatLossless:
		formatLabel = "Lossless"
	case core.FormatLossy:
		formatLabel = "Lossy"
	}
	parts = append(parts, "[f] Format: "+formatLabel)

	// No slot filter
	slotLabel := filterOff
	if m.filterNoSlot {
		slotLabel = filterOn
	}
	parts = append(parts, "[s] No slot: "+slotLabel)

	// Track count filter
	trackLabel := filterOff
	if m.filterTrackCount {
		trackLabel = filterOn
	}
	parts = append(parts, "[t] Track count: "+trackLabel)

	return dimStyle().Render(strings.Join(parts, "  |  "))
}

// renderFilterStats renders the filter statistics.
func (m *Model) renderFilterStats() string {
	s := m.filterStats

	var parts []string

	// Show what was filtered out
	if s.NoFreeSlot > 0 {
		parts = append(parts, fmt.Sprintf("no slot: %d", s.NoFreeSlot))
	}
	if s.NoAudioFiles > 0 {
		parts = append(parts, fmt.Sprintf("no audio: %d", s.NoAudioFiles))
	}
	if s.WrongFormat > 0 {
		parts = append(parts, fmt.Sprintf("wrong format: %d", s.WrongFormat))
	}
	if s.WrongTrackCount > 0 {
		parts = append(parts, fmt.Sprintf("≠%d tracks: %d", s.ExpectedTracks, s.WrongTrackCount))
	}

	if len(parts) == 0 {
		return dimStyle().Render("Filtered: none")
	}

	result := "Filtered: " + strings.Join(parts, ", ")
	return dimStyle().Render(result)
}

// formatBitRate formats bitrate for display.
// Returns "-" if bitrate is 0 (typically lossless formats).
func formatBitRate(kbps int) string {
	if kbps == 0 {
		return "-"
	}
	return strconv.Itoa(kbps)
}

// formatSpeed formats upload speed in human-readable form.
// Uses binary calculation (1024) with SI notation (KB, MB).
func formatSpeed(bytesPerSec int) string {
	return formatSize(int64(bytesPerSec)) + "/s"
}
