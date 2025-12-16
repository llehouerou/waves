// Package scanreport provides a popup component for displaying library scan results.
package scanreport

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/library"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// Compile-time check that Model implements popup.Popup.
var _ popup.Popup = (*Model)(nil)

// DefaultMaxExamples is the number of example track paths to show per category.
const DefaultMaxExamples = 3

// Model holds the state for the scan report popup.
type Model struct {
	Stats       *library.ScanStats
	MaxExamples int
	Width       int
	Height      int
}

// New creates a new scan report model.
func New(stats *library.ScanStats) Model {
	return Model{
		Stats:       stats,
		MaxExamples: DefaultMaxExamples,
	}
}

// SetSize implements popup.Popup.
func (m *Model) SetSize(width, height int) {
	m.Width = width
	m.Height = height
}

// Init implements popup.Popup.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements popup.Popup.
func (m *Model) Update(_ tea.Msg) (popup.Popup, tea.Cmd) {
	// ScanReport doesn't handle any messages - it's closed by the manager
	return m, nil
}

// View implements popup.Popup.
func (m *Model) View() string {
	if m.Stats == nil {
		return ""
	}

	content := m.buildContent()

	p := popup.New()
	p.Title = "Library Scan Complete"
	p.Content = content
	p.Footer = "Press Enter or Escape to close"

	return p.Render(m.Width, m.Height)
}

func (m Model) buildContent() string {
	var sb strings.Builder

	// Sort sources for consistent output
	sources := make([]string, 0, len(m.Stats.BySource))
	for src := range m.Stats.BySource {
		sources = append(sources, src)
	}
	sort.Strings(sources)

	// Calculate totals
	var totalAdded, totalRemoved, totalUpdated int
	for _, stats := range m.Stats.BySource {
		totalAdded += len(stats.Added)
		totalRemoved += len(stats.Removed)
		totalUpdated += len(stats.Updated)
	}

	// Render each source
	for i, src := range sources {
		if i > 0 {
			sb.WriteString("\n")
		}

		stats := m.Stats.BySource[src]
		sourceStyle := lipgloss.NewStyle().Bold(true)
		sb.WriteString(sourceStyle.Render(src))
		sb.WriteString("\n")

		hasChanges := len(stats.Added) > 0 || len(stats.Removed) > 0 || len(stats.Updated) > 0

		if !hasChanges {
			dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			sb.WriteString("  ")
			sb.WriteString(dimStyle.Render("No changes"))
			sb.WriteString("\n")
			continue
		}

		// Added
		if len(stats.Added) > 0 {
			m.renderCategory(&sb, "Added", stats.Added, lipgloss.Color("42"))
		}

		// Removed
		if len(stats.Removed) > 0 {
			m.renderCategory(&sb, "Removed", stats.Removed, lipgloss.Color("196"))
		}

		// Updated
		if len(stats.Updated) > 0 {
			m.renderCategory(&sb, "Updated", stats.Updated, lipgloss.Color("214"))
		}
	}

	// Total line
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteString("\n")

	totalStyle := lipgloss.NewStyle().Bold(true)
	sb.WriteString(totalStyle.Render(fmt.Sprintf("Total: %d added, %d removed, %d updated",
		totalAdded, totalRemoved, totalUpdated)))

	return sb.String()
}

func (m Model) renderCategory(sb *strings.Builder, label string, paths []string, color lipgloss.Color) {
	labelStyle := lipgloss.NewStyle().Foreground(color)
	sb.WriteString("  ")
	sb.WriteString(labelStyle.Render(fmt.Sprintf("%s: %d", label, len(paths))))
	sb.WriteString("\n")

	// Show examples
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	for i, path := range paths {
		if i >= m.MaxExamples {
			remaining := len(paths) - m.MaxExamples
			sb.WriteString("    ")
			sb.WriteString(dimStyle.Render(fmt.Sprintf("... and %d more", remaining)))
			sb.WriteString("\n")
			break
		}
		sb.WriteString("    • ")
		sb.WriteString(dimStyle.Render(path))
		sb.WriteString("\n")
	}
}
