package download

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/render"
)

// renderSlskdResults renders the slskd search results as a table.
func (m *Model) renderSlskdResults() string {
	if len(m.slskdResults) == 0 {
		// Don't show "no sources" while still searching
		if m.state == StateSlskdSearching {
			return ""
		}
		return dimStyle().Render("No sources found")
	}

	var b strings.Builder
	b.WriteString(dimStyle().Render("Select a download source:"))
	b.WriteString("\n\n")

	columns := m.slskdColumns()
	header := slskdRow(columns, func(c slskdColumn) string { return c.title })
	b.WriteString(dimStyle().Render("  " + header))
	b.WriteString("\n")
	b.WriteString(render.Separator(2 + lipgloss.Width(header)))
	b.WriteString("\n")

	maxVisible := m.slskdListHeight()
	start, end := m.slskdCursor.VisibleRange(len(m.slskdResults), maxVisible)
	cursorPos := m.slskdCursor.Pos()

	for i := start; i < end; i++ {
		r := &m.slskdResults[i]
		row := slskdRow(columns, func(c slskdColumn) string { return c.value(r) })

		if i == cursorPos {
			b.WriteString(cursorStyle().Render("> "))
			b.WriteString(selectedStyle().Render(row))
		} else {
			b.WriteString(dimStyle().Render("  " + row))
		}
		b.WriteString("\n")
	}

	// Show filter controls
	b.WriteString("\n")
	b.WriteString(m.renderFilterControls())

	// Show filter stats
	b.WriteString("\n")
	b.WriteString(m.renderFilterStats())

	return b.String()
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

// slskdColumns returns the columns of the slskd results table: a Directory
// column in the wide layout only.
func (m *Model) slskdColumns() []slskdColumn {
	const (
		colUser    = 18
		colFormat  = 8
		colBitRate = 6
		colFiles   = 5
		colSize    = 9
		colSpeed   = 10
	)
	columns := []slskdColumn{{title: "User", width: colUser, value: func(r *core.SlskdResult) string {
		return render.Cut(r.Username, colUser)
	}}}
	if !m.isCompactMode() {
		const (
			fixedWidth  = colUser + colFormat + colBitRate + colFiles + colSize + colSpeed + 12
			minDirWidth = 20
			maxDirWidth = 50
		)
		colDir := min(max(m.Width()-fixedWidth, minDirWidth), maxDirWidth)
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
