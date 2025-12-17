package download

import (
	"fmt"
	"strconv"
	"strings"
)

// renderSlskdResults renders the slskd search results as a table.
func (m *Model) renderSlskdResults() string {
	if len(m.slskdResults) == 0 {
		// Don't show "no sources" while still searching
		if m.state == StateSlskdSearching {
			return ""
		}
		return dimStyle.Render("No sources found")
	}

	var b strings.Builder
	b.WriteString(dimStyle.Render("Select a download source:"))
	b.WriteString("\n\n")

	// Column widths - fixed columns plus dynamic directory
	const (
		colUser     = 18
		colFormat   = 8
		colBitRate  = 6
		colFiles    = 5
		colSize     = 9
		colSpeed    = 10
		fixedWidth  = colUser + colFormat + colBitRate + colFiles + colSize + colSpeed + 12 // spacing + cursor
		minDirWidth = 20
		maxDirWidth = 50
	)
	// Directory gets remaining space, clamped to min/max
	colDir := min(max(m.width-fixedWidth, minDirWidth), maxDirWidth)

	// Header
	header := fmt.Sprintf("  %-*s %-*s %-*s %*s %*s %*s %*s",
		colUser, "User",
		colDir, "Directory",
		colFormat, "Format",
		colBitRate, "kbps",
		colFiles, "Files",
		colSize, "Size",
		colSpeed, "Speed")
	b.WriteString(dimStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", colUser+colDir+colFormat+colBitRate+colFiles+colSize+colSpeed+9)))
	b.WriteString("\n")

	maxVisible := max(m.height-14, 5)
	start, end := m.slskdCursor.VisibleRange(len(m.slskdResults), maxVisible)
	cursorPos := m.slskdCursor.Pos()

	for i := start; i < end; i++ {
		r := &m.slskdResults[i]

		// Format each column
		user := truncateName(r.Username, colUser)
		dir := truncateDirectory(r.Directory, colDir)
		format := r.Format
		bitrate := formatBitRate(r.BitRate)
		files := strconv.Itoa(r.FileCount)
		size := formatSize(r.TotalSize)
		speed := formatSpeed(r.UploadSpeed)

		// Build row
		row := fmt.Sprintf("%-*s %-*s %-*s %*s %*s %*s %*s",
			colUser, user,
			colDir, dir,
			colFormat, format,
			colBitRate, bitrate,
			colFiles, files,
			colSize, size,
			colSpeed, speed)

		if i == cursorPos {
			b.WriteString(cursorStyle.Render("> "))
			b.WriteString(selectedStyle.Render(row))
		} else {
			b.WriteString("  ")
			b.WriteString(row)
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

// renderFilterControls renders the current filter settings.
func (m *Model) renderFilterControls() string {
	var parts []string

	// Format filter
	var formatLabel string
	switch m.formatFilter {
	case FormatBoth:
		formatLabel = "Both"
	case FormatLossless:
		formatLabel = "Lossless"
	case FormatLossy:
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

	return dimStyle.Render(strings.Join(parts, "  |  "))
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
		return dimStyle.Render("Filtered: none")
	}

	result := "Filtered: " + strings.Join(parts, ", ")
	return dimStyle.Render(result)
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
