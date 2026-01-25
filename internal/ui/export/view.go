package export

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/llehouerou/waves/internal/export"
	"github.com/llehouerou/waves/internal/ui/styles"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(styles.T().Primary)

	selectedStyle = lipgloss.NewStyle().
			Foreground(styles.T().Primary).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(styles.T().FgMuted)
)

// View renders the popup.
func (m Model) View() string {
	switch m.state {
	case StateSelectTarget:
		return m.viewSelectTarget()
	case StateNewTarget:
		return m.viewNewTarget()
	case StateNewTargetFolder:
		return m.viewNewTargetFolder()
	case StateNewTargetConfig:
		return m.viewNewTargetConfig()
	case StateRenameTarget:
		return m.viewRenameTarget()
	case StateCustomFolder:
		return m.viewCustomFolder()
	case StateCustomFolderConfig:
		return m.viewCustomFolderConfig()
	}
	return ""
}

//nolint:goconst // UI prefix strings are clearer inline
func (m Model) viewSelectTarget() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("Export"))
	b.WriteString("\n\n")

	// What we're exporting
	if m.albumName != "" {
		b.WriteString(fmt.Sprintf("  %s\n", m.albumName))
	}
	b.WriteString(fmt.Sprintf("  %d tracks\n\n", len(m.tracks)))

	// Target list
	b.WriteString("  Target:\n")
	for i, target := range m.targets {
		prefix := "    "
		if i == m.selectedIdx {
			prefix = "  ▸ "
		}

		connected := m.isTargetConnected(target)
		line := target.Name
		if !connected {
			line = dimStyle.Render(line + " (not connected)")
		}

		if i == m.selectedIdx {
			line = selectedStyle.Render(line)
		}

		b.WriteString(prefix + line + "\n")
	}

	// New target option
	newTargetLine := "+ New target..."
	if m.selectedIdx == len(m.targets) {
		newTargetLine = selectedStyle.Render(newTargetLine)
		b.WriteString("  ▸ " + newTargetLine + "\n")
	} else {
		b.WriteString("    " + newTargetLine + "\n")
	}

	// FLAC conversion toggle
	if m.HasFLAC() {
		b.WriteString("\n")
		checkbox := "[ ]"
		if m.convertFLAC {
			checkbox = "[x]"
		}
		b.WriteString(fmt.Sprintf("  %s Convert FLAC to MP3 (%d files, 320kbps)\n",
			checkbox, m.flacCount))
	}

	// Help
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑↓ navigate  enter confirm  r rename  d delete  space toggle  esc cancel"))

	return b.String()
}

func (m Model) viewNewTarget() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Select Device"))
	b.WriteString("\n\n")

	// Show volumes
	for i, vol := range m.volumes {
		prefix := "    "
		if i == m.volumeIdx {
			prefix = "  ▸ "
		}

		line := vol.String()
		if i == m.volumeIdx {
			line = selectedStyle.Render(line)
		}
		b.WriteString(prefix + line + "\n")
	}

	// Custom folder option (always available)
	customFolderLine := "📁 Custom folder..."
	if m.volumeIdx == len(m.volumes) {
		customFolderLine = selectedStyle.Render(customFolderLine)
		b.WriteString("  ▸ " + customFolderLine + "\n")
	} else {
		b.WriteString("    " + customFolderLine + "\n")
	}

	if len(m.volumes) == 0 {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  No removable devices detected.") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑↓ navigate  enter select  esc back"))

	return b.String()
}

func (m Model) viewNewTargetFolder() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Select Folder"))
	b.WriteString("\n\n")

	// Show device and current path
	label := m.newTarget.DeviceLabel
	if label == "" {
		label = m.newTarget.Name
	}
	b.WriteString(fmt.Sprintf("  Device: %s\n", label))
	b.WriteString(fmt.Sprintf("  Path: %s\n\n", m.currentPath))

	// Directory listing
	hasParent := m.currentPath != "/"
	idx := 0

	// Show ".." if not at root
	if hasParent {
		prefix := "    "
		line := ".."
		if m.dirIdx == idx {
			prefix = "  ▸ "
			line = selectedStyle.Render(line)
		}
		b.WriteString(prefix + line + "\n")
		idx++
	}

	// Show directories
	for _, dir := range m.directories {
		prefix := "    "
		line := dir
		if m.dirIdx == idx {
			prefix = "  ▸ "
			line = selectedStyle.Render(line)
		}
		b.WriteString(prefix + line + "\n")
		idx++
	}

	// Empty directory message
	if len(m.directories) == 0 && !hasParent {
		b.WriteString(dimStyle.Render("  (empty)") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑↓ navigate  enter open  space select  backspace up  esc back"))

	return b.String()
}

func (m Model) viewNewTargetConfig() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Configure Target"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  Device: %s\n", m.newTarget.DeviceLabel))
	b.WriteString(fmt.Sprintf("  Folder: %s\n\n", m.newTarget.Subfolder))

	b.WriteString("  Folder structure:\n")
	structures := []struct {
		fs    export.FolderStructure
		label string
	}{
		{export.FolderStructureFlat, "Flat (Artist - Album/Track)"},
		{export.FolderStructureHierarchical, "Hierarchical (Artist/Album/Track)"},
		{export.FolderStructureSingle, "Single folder (all files flat)"},
	}

	for i, s := range structures {
		prefix := "    "
		if i == m.structureIdx {
			prefix = "  ▸ "
		}
		line := fmt.Sprintf("[%d] %s", i+1, s.label)
		if i == m.structureIdx {
			line = selectedStyle.Render(line)
		}
		b.WriteString(prefix + line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑↓/1-3 select  enter save  esc back"))

	return b.String()
}

func (m Model) viewRenameTarget() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Rename Target"))
	b.WriteString("\n\n")

	// Show current name with cursor
	b.WriteString("  New name:\n")
	b.WriteString("  " + selectedStyle.Render(m.renameInput+"▏") + "\n")

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  enter confirm  esc cancel"))

	return b.String()
}

func (m Model) viewCustomFolder() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Custom Folder"))
	b.WriteString("\n\n")

	b.WriteString("  Enter folder path:\n")
	b.WriteString("  " + selectedStyle.Render(m.customFolderInput+"▏") + "\n")

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  enter confirm  esc back"))

	return b.String()
}

func (m Model) viewCustomFolderConfig() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Configure Target"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  Folder: %s\n\n", m.newTarget.Subfolder))

	b.WriteString("  Folder structure:\n")
	structures := []struct {
		fs    export.FolderStructure
		label string
	}{
		{export.FolderStructureFlat, "Flat (Artist - Album/Track)"},
		{export.FolderStructureHierarchical, "Hierarchical (Artist/Album/Track)"},
		{export.FolderStructureSingle, "Single folder (all files flat)"},
	}

	for i, s := range structures {
		prefix := "    "
		if i == m.structureIdx {
			prefix = "  ▸ "
		}
		line := fmt.Sprintf("[%d] %s", i+1, s.label)
		if i == m.structureIdx {
			line = selectedStyle.Render(line)
		}
		b.WriteString(prefix + line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  ↑↓/1-3 select  enter save  esc back"))

	return b.String()
}
