package export

import (
	"fmt"
	"strings"

	"github.com/llehouerou/waves/internal/export"
	"github.com/llehouerou/waves/internal/ui/render"
	"github.com/llehouerou/waves/internal/ui/styles"
)

var (
	titleStyle = styles.T().BaseStyle().
			Bold(true).
			Foreground(styles.T().Primary)

	selectedStyle = styles.T().BaseStyle().
			Foreground(styles.T().Primary).
			Bold(true)

	dimStyle = styles.T().BaseStyle().
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
		b.WriteString(render.EmptyLine(2) + dimStyle.Render(m.albumName) + "\n")
	}
	b.WriteString(render.EmptyLine(2) + dimStyle.Render(fmt.Sprintf("%d tracks", len(m.tracks))) + "\n\n")

	// Target list
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("Target:") + "\n")
	for i, target := range m.targets {
		connected := m.isTargetConnected(target)
		line := target.Name
		if !connected {
			line = dimStyle.Render(line + " (not connected)")
		}

		if i == m.selectedIdx {
			if connected {
				line = selectedStyle.Render(line)
			}
			b.WriteString(render.EmptyLine(2) + selectedStyle.Render("▸ ") + line + "\n")
		} else {
			b.WriteString(render.EmptyLine(4) + line + "\n")
		}
	}

	// New target option
	if m.selectedIdx == len(m.targets) {
		b.WriteString(render.EmptyLine(2) + selectedStyle.Render("▸ + New target...") + "\n")
	} else {
		b.WriteString(render.EmptyLine(4) + dimStyle.Render("+ New target...") + "\n")
	}

	// FLAC conversion toggle
	if m.HasFLAC() {
		b.WriteString("\n")
		checkbox := "[ ]"
		if m.convertFLAC {
			checkbox = "[x]"
		}
		b.WriteString(render.EmptyLine(2) + dimStyle.Render(fmt.Sprintf("%s Convert FLAC to MP3 (%d files, 320kbps)", checkbox, m.flacCount)) + "\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("↑↓ navigate  enter confirm  r rename  d delete  space toggle  esc cancel"))

	return b.String()
}

func (m Model) viewNewTarget() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Select Device"))
	b.WriteString("\n\n")

	// Show volumes
	for i, vol := range m.volumes {
		line := vol.String()
		if i == m.volumeIdx {
			line = selectedStyle.Render(line)
			b.WriteString(render.EmptyLine(2) + selectedStyle.Render("▸ ") + line + "\n")
		} else {
			b.WriteString(render.EmptyLine(4) + dimStyle.Render(line) + "\n")
		}
	}

	// Custom folder option (always available)
	if m.volumeIdx == len(m.volumes) {
		b.WriteString(render.EmptyLine(2) + selectedStyle.Render("▸ 📁 Custom folder...") + "\n")
	} else {
		b.WriteString(render.EmptyLine(4) + dimStyle.Render("📁 Custom folder...") + "\n")
	}

	if len(m.volumes) == 0 {
		b.WriteString("\n")
		b.WriteString(render.EmptyLine(2) + dimStyle.Render("No removable devices detected.") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("↑↓ navigate  enter select  esc back"))

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
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("Device: "+label) + "\n")
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("Path: "+m.currentPath) + "\n\n")

	// Directory listing
	hasParent := m.currentPath != "/"
	idx := 0

	// Show ".." if not at root
	if hasParent {
		if m.dirIdx == idx {
			b.WriteString(render.EmptyLine(2) + selectedStyle.Render("▸ ..") + "\n")
		} else {
			b.WriteString(render.EmptyLine(4) + dimStyle.Render("..") + "\n")
		}
		idx++
	}

	// Show directories
	for _, dir := range m.directories {
		if m.dirIdx == idx {
			b.WriteString(render.EmptyLine(2) + selectedStyle.Render("▸ "+dir) + "\n")
		} else {
			b.WriteString(render.EmptyLine(4) + dimStyle.Render(dir) + "\n")
		}
		idx++
	}

	// Empty directory message
	if len(m.directories) == 0 && !hasParent {
		b.WriteString(render.EmptyLine(2) + dimStyle.Render("(empty)") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("↑↓ navigate  enter open  space select  backspace up  esc back"))

	return b.String()
}

func (m Model) viewNewTargetConfig() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Configure Target"))
	b.WriteString("\n\n")

	b.WriteString(render.EmptyLine(2) + dimStyle.Render("Device: "+m.newTarget.DeviceLabel) + "\n")
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("Folder: "+m.newTarget.Subfolder) + "\n\n")

	b.WriteString(render.EmptyLine(2) + dimStyle.Render("Folder structure:") + "\n")
	structures := []struct {
		fs    export.FolderStructure
		label string
	}{
		{export.FolderStructureFlat, "Flat (Artist - Album/Track)"},
		{export.FolderStructureHierarchical, "Hierarchical (Artist/Album/Track)"},
		{export.FolderStructureSingle, "Single folder (all files flat)"},
	}

	for i, s := range structures {
		line := fmt.Sprintf("[%d] %s", i+1, s.label)
		if i == m.structureIdx {
			b.WriteString(render.EmptyLine(2) + selectedStyle.Render("▸ "+line) + "\n")
		} else {
			b.WriteString(render.EmptyLine(4) + dimStyle.Render(line) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("↑↓/1-3 select  enter save  esc back"))

	return b.String()
}

func (m Model) viewRenameTarget() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Rename Target"))
	b.WriteString("\n\n")

	// Show current name with cursor
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("New name:") + "\n")
	b.WriteString(render.EmptyLine(2) + selectedStyle.Render(m.renameInput+"▏") + "\n")

	b.WriteString("\n")
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("enter confirm  esc cancel"))

	return b.String()
}

func (m Model) viewCustomFolder() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Custom Folder"))
	b.WriteString("\n\n")

	b.WriteString(render.EmptyLine(2) + dimStyle.Render("Enter folder path:") + "\n")
	b.WriteString(render.EmptyLine(2) + selectedStyle.Render(m.customFolderInput+"▏") + "\n")

	b.WriteString("\n")
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("enter confirm  esc back"))

	return b.String()
}

func (m Model) viewCustomFolderConfig() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Configure Target"))
	b.WriteString("\n\n")

	b.WriteString(render.EmptyLine(2) + dimStyle.Render("Folder: "+m.newTarget.Subfolder) + "\n\n")

	b.WriteString(render.EmptyLine(2) + dimStyle.Render("Folder structure:") + "\n")
	structures := []struct {
		fs    export.FolderStructure
		label string
	}{
		{export.FolderStructureFlat, "Flat (Artist - Album/Track)"},
		{export.FolderStructureHierarchical, "Hierarchical (Artist/Album/Track)"},
		{export.FolderStructureSingle, "Single folder (all files flat)"},
	}

	for i, s := range structures {
		line := fmt.Sprintf("[%d] %s", i+1, s.label)
		if i == m.structureIdx {
			b.WriteString(render.EmptyLine(2) + selectedStyle.Render("▸ "+line) + "\n")
		} else {
			b.WriteString(render.EmptyLine(4) + dimStyle.Render(line) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(render.EmptyLine(2) + dimStyle.Render("↑↓/1-3 select  enter save  esc back"))

	return b.String()
}
