// Package scanreport provides a popup component for displaying library scan results.
package scanreport

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui"
	"github.com/llehouerou/waves/internal/ui/popup"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

// Compile-time check that Model implements popup.Popup.
var _ popup.Popup = (*Model)(nil)

// DefaultMaxExamples is the number of example track paths to show per category.
const DefaultMaxExamples = 3

// Model holds the state for the scan report popup.
type Model struct {
	ui.Base
	Stats       *core.ScanStats
	MaxExamples int
}

// New creates a new scan report model.
func New(stats *core.ScanStats) Model {
	return Model{
		Stats:       stats,
		MaxExamples: DefaultMaxExamples,
	}
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

	titleStyle := styles.T().S().Title
	footerStyle := styles.T().S().Subtle

	var result strings.Builder
	result.WriteString(titleStyle.Render("Library Scan Complete"))
	result.WriteString("\n\n")
	result.WriteString(content)
	result.WriteString("\n\n")
	result.WriteString(footerStyle.Render("Press Enter or Escape to close"))

	return result.String()
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
	var totalAdded, totalRemoved, totalUpdated, totalUnread int
	for _, stats := range m.Stats.BySource {
		totalAdded += len(stats.Added)
		totalRemoved += len(stats.Removed)
		totalUpdated += len(stats.Updated)
		totalUnread += len(stats.Unread)
	}

	// Render each source
	for i, src := range sources {
		if i > 0 {
			sb.WriteString("\n")
		}

		stats := m.Stats.BySource[src]
		sourceStyle := styles.T().BaseStyle().Bold(true)
		sb.WriteString(sourceStyle.Render(src))
		sb.WriteString("\n")

		hasChanges := len(stats.Added) > 0 || len(stats.Removed) > 0 || len(stats.Updated) > 0 ||
			len(stats.Unread) > 0

		t := styles.T()
		if !hasChanges {
			dimStyle := t.S().Subtle
			sb.WriteString(render.EmptyLine(2))
			sb.WriteString(dimStyle.Render("No changes"))
			sb.WriteString("\n")
			continue
		}

		// Added
		if len(stats.Added) > 0 {
			m.renderCategory(&sb, "Added", stats.Added, t.Success, m.MaxExamples)
		}

		// Removed
		if len(stats.Removed) > 0 {
			m.renderCategory(&sb, "Removed", stats.Removed, t.Error, m.MaxExamples)
		}

		// Updated
		if len(stats.Updated) > 0 {
			m.renderCategory(&sb, "Updated", stats.Updated, t.Warning, m.MaxExamples)
		}

		// Unread: the scan kept their tracks
		if len(stats.Unread) > 0 {
			// All of them: the user must know what the scan did not look at.
			m.renderCategory(&sb, "Not read, tracks kept", unreadNames(stats.Unread), t.Warning, len(stats.Unread))
		}
	}

	// Total line
	sb.WriteString("\n")
	sb.WriteString(render.Separator(40))
	sb.WriteString("\n")

	totalStyle := styles.T().BaseStyle().Bold(true)
	total := fmt.Sprintf("Total: %d added, %d removed, %d updated", totalAdded, totalRemoved, totalUpdated)
	if totalUnread > 0 {
		total += fmt.Sprintf(", %d not read", totalUnread)
	}
	sb.WriteString(totalStyle.Render(total))

	return sb.String()
}

// unreadNames names the unread paths of a source.
func unreadNames(paths []string) []string {
	names := make([]string, len(paths))
	for i, path := range paths {
		names[i] = path
		if path == core.SourceRoot {
			names[i] = "the source folder"
		}
	}
	return names
}

// renderCategory renders label with the count of paths and up to maxShown of them.
func (m Model) renderCategory(sb *strings.Builder, label string, paths []string, color lipgloss.Color, maxShown int) {
	labelStyle := styles.T().BaseStyle().Foreground(color)
	sb.WriteString(render.EmptyLine(2))
	sb.WriteString(labelStyle.Render(fmt.Sprintf("%s: %d", label, len(paths))))
	sb.WriteString("\n")

	// Show examples
	dimStyle := styles.T().S().Subtle
	for i, path := range paths {
		if i >= maxShown {
			remaining := len(paths) - maxShown
			sb.WriteString(render.EmptyLine(4))
			sb.WriteString(dimStyle.Render(fmt.Sprintf("... and %d more", remaining)))
			sb.WriteString("\n")
			break
		}
		sb.WriteString(render.EmptyLine(4))
		sb.WriteString(styles.T().S().Muted.Render("• "))
		sb.WriteString(dimStyle.Render(path))
		sb.WriteString("\n")
	}
}
