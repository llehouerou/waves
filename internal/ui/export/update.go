package export

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/export"
	"github.com/llehouerou/waves/internal/ui/popup"
)

// expandTilde expands ~ to the user's home directory.
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	return path
}

// Init initializes the popup.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		LoadVolumesCmd(),
		LoadTargetsCmd(m.repo),
	)
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case VolumesLoadedMsg:
		if msg.Err == nil {
			m.volumes = msg.Volumes
			m.autoSelectTarget()
		}
		return m, nil

	case TargetsLoadedMsg:
		if msg.Err == nil {
			m.targets = msg.Targets
			m.autoSelectTarget()
		}
		return m, nil

	case TargetCreatedMsg:
		if msg.Err == nil {
			m.targets = append(m.targets, msg.Target)
			m.selectedIdx = len(m.targets) - 1
			m.state = StateSelectTarget
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch m.state {
	case StateSelectTarget:
		return m.handleSelectTargetKey(msg)
	case StateNewTarget:
		return m.handleNewTargetKey(msg)
	case StateNewTargetFolder:
		return m.handleNewTargetFolderKey(msg)
	case StateNewTargetConfig:
		return m.handleNewTargetConfigKey(msg)
	case StateRenameTarget:
		return m.handleRenameTargetKey(msg)
	case StateCustomFolder:
		return m.handleCustomFolderKey(msg)
	case StateCustomFolderConfig:
		return m.handleCustomFolderConfigKey(msg)
	}
	return m, nil
}

//nolint:goconst // Key strings are clearer inline
func (m *Model) handleSelectTargetKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return m, func() tea.Msg { return ActionMsg(Close{}) }

	case "up", "k":
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}

	case "down", "j":
		maxIdx := len(m.targets) // +1 for "New target" option
		if m.selectedIdx < maxIdx {
			m.selectedIdx++
		}

	case "d":
		// Delete selected target
		target, ok := m.SelectedTarget()
		if ok {
			return m, func() tea.Msg {
				return ActionMsg(DeleteTarget{ID: target.ID, Name: target.Name})
			}
		}

	case "r":
		// Rename selected target
		target, ok := m.SelectedTarget()
		if ok {
			m.renameTargetID = target.ID
			m.renameInput = target.Name
			m.state = StateRenameTarget
		}

	case " ":
		// Toggle FLAC conversion
		if m.HasFLAC() {
			m.convertFLAC = !m.convertFLAC
		}

	case "enter":
		if m.selectedIdx == len(m.targets) {
			// "New target" selected - show device selection screen
			// (will show "no devices" message if none available)
			m.state = StateNewTarget
			m.volumeIdx = 0
			return m, nil
		}

		target, ok := m.SelectedTarget()
		if !ok {
			return m, nil
		}

		// Validate we have tracks to export
		if len(m.tracks) == 0 {
			return m, nil
		}

		// Determine base path for export
		var basePath string
		if m.isCustomFolderTarget(target) {
			// Custom folder target - use Subfolder directly
			basePath = target.Subfolder
		} else {
			// Device-based target - find mount path
			basePath = m.findMountPath(target.DeviceUUID)
			if basePath == "" {
				// Device not connected - show error
				return m, func() tea.Msg {
					return ActionMsg(DeviceNotConnected{TargetName: target.Name})
				}
			}
		}

		return m, func() tea.Msg {
			return ActionMsg(StartExport{
				Target:      target,
				Tracks:      m.tracks,
				ConvertFLAC: m.convertFLAC,
				MountPath:   basePath,
			})
		}
	}

	return m, nil
}

func (m *Model) handleNewTargetKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	// volumeIdx can go from 0 to len(volumes) where len(volumes) is the "Custom folder" option
	maxIdx := len(m.volumes)

	switch msg.String() {
	case "esc":
		m.state = StateSelectTarget
		return m, nil

	case "up", "k":
		if m.volumeIdx > 0 {
			m.volumeIdx--
		}

	case "down", "j":
		if m.volumeIdx < maxIdx {
			m.volumeIdx++
		}

	case "enter":
		// Check if "Custom folder" is selected
		if m.volumeIdx == len(m.volumes) {
			m.customFolderInput = ""
			m.state = StateCustomFolder
			return m, nil
		}

		if len(m.volumes) == 0 {
			return m, nil
		}
		vol := m.volumes[m.volumeIdx]
		// Use label, or fall back to mount path basename, or UUID
		name := vol.Label
		if name == "" {
			name = filepath.Base(vol.MountPath)
		}
		if name == "" || name == "/" {
			name = vol.UUID
		}
		m.newTarget = export.Target{
			DeviceUUID:      vol.UUID,
			DeviceLabel:     vol.Label,
			Name:            name,
			Subfolder:       "/",
			FolderStructure: export.FolderStructureFlat,
		}
		m.state = StateNewTargetFolder
	}

	return m, nil
}

func (m *Model) handleNewTargetFolderKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	// Simplified: just accept root folder for now
	switch msg.String() {
	case "esc":
		m.state = StateNewTarget
		return m, nil

	case "enter":
		m.structureIdx = 0
		m.folderStructure = export.FolderStructureFlat
		m.state = StateNewTargetConfig
	}

	return m, nil
}

func (m *Model) handleNewTargetConfigKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = StateNewTargetFolder
		return m, nil

	case "up", "k":
		if m.structureIdx > 0 {
			m.structureIdx--
			m.updateFolderStructure()
		}

	case "down", "j":
		if m.structureIdx < 2 {
			m.structureIdx++
			m.updateFolderStructure()
		}

	case "1":
		m.structureIdx = 0
		m.folderStructure = export.FolderStructureFlat

	case "2":
		m.structureIdx = 1
		m.folderStructure = export.FolderStructureHierarchical

	case "3":
		m.structureIdx = 2
		m.folderStructure = export.FolderStructureSingle

	case "enter":
		m.newTarget.FolderStructure = m.folderStructure
		return m, CreateTargetCmd(m.repo, m.newTarget)
	}

	return m, nil
}

func (m *Model) updateFolderStructure() {
	switch m.structureIdx {
	case 0:
		m.folderStructure = export.FolderStructureFlat
	case 1:
		m.folderStructure = export.FolderStructureHierarchical
	case 2:
		m.folderStructure = export.FolderStructureSingle
	}
}

func (m *Model) handleRenameTargetKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = StateSelectTarget
		m.renameInput = ""
		return m, nil

	case "enter":
		if m.renameInput == "" {
			return m, nil
		}
		id := m.renameTargetID
		newName := m.renameInput
		m.state = StateSelectTarget
		m.renameInput = ""
		return m, func() tea.Msg {
			return ActionMsg(RenameTarget{ID: id, NewName: newName})
		}

	case "backspace":
		if m.renameInput != "" {
			m.renameInput = m.renameInput[:len(m.renameInput)-1]
		}

	default:
		// Add printable characters
		if len(msg.String()) == 1 {
			m.renameInput += msg.String()
		}
	}

	return m, nil
}

func (m *Model) handleCustomFolderKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = StateNewTarget
		m.customFolderInput = ""
		return m, nil

	case "enter":
		if m.customFolderInput == "" {
			return m, nil
		}
		// Expand tilde and use folder basename as the target name
		expandedPath := expandTilde(m.customFolderInput)
		name := filepath.Base(expandedPath)
		if name == "" || name == "/" || name == "." {
			name = "Custom"
		}
		m.newTarget = export.Target{
			DeviceUUID:      "", // Empty = custom folder target
			DeviceLabel:     "",
			Name:            name,
			Subfolder:       expandedPath,
			FolderStructure: export.FolderStructureFlat,
		}
		m.structureIdx = 0
		m.folderStructure = export.FolderStructureFlat
		m.state = StateCustomFolderConfig
		return m, nil

	case "backspace":
		if m.customFolderInput != "" {
			m.customFolderInput = m.customFolderInput[:len(m.customFolderInput)-1]
		}

	default:
		// Add printable characters
		if len(msg.String()) == 1 {
			m.customFolderInput += msg.String()
		}
	}

	return m, nil
}

func (m *Model) handleCustomFolderConfigKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = StateCustomFolder
		return m, nil

	case "up", "k":
		if m.structureIdx > 0 {
			m.structureIdx--
			m.updateFolderStructure()
		}

	case "down", "j":
		if m.structureIdx < 2 {
			m.structureIdx++
			m.updateFolderStructure()
		}

	case "1":
		m.structureIdx = 0
		m.folderStructure = export.FolderStructureFlat

	case "2":
		m.structureIdx = 1
		m.folderStructure = export.FolderStructureHierarchical

	case "3":
		m.structureIdx = 2
		m.folderStructure = export.FolderStructureSingle

	case "enter":
		m.newTarget.FolderStructure = m.folderStructure
		return m, CreateTargetCmd(m.repo, m.newTarget)
	}

	return m, nil
}
