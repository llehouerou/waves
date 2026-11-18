package export

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/errmsg"
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

// Init reads the volumes and the targets.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.volumesRead.Ask(loadVolumes(m.exports)),
		m.targetsRead.Ask(loadTargets(m.exports)),
	)
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (popup.Popup, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case VolumesLoadedMsg:
		if !m.volumesRead.Take(msg.Request) {
			return m, nil
		}
		m.volumesFailed = errmsg.Format(errmsg.OpVolumeDetect, msg.Err)
		if msg.Err == nil {
			m.volumes = msg.Volumes
			m.autoSelectTarget()
		}
		return m, nil

	case TargetsLoadedMsg:
		if !m.targetsRead.Take(msg.Request) {
			return m, nil
		}
		saved := m.savedID
		m.savedID = 0
		m.targetsFailed = errmsg.Format(errmsg.OpTargetsLoad, msg.Err)
		if msg.Err == nil {
			was, onTarget := m.SelectedTarget()
			m.targets = msg.Targets
			if m.listed {
				m.keepCursor(was, onTarget)
			} else {
				m.autoSelectTarget()
			}
			m.listed = true
			if saved != 0 {
				m.selectTarget(saved)
			}
		}
		return m, nil

	case TargetCreatedMsg:
		if !m.saving.Take(msg.Request) {
			return m, nil
		}
		if msg.Err != nil {
			m.failed = errmsg.Format(errmsg.OpTargetCreate, msg.Err)
			return m, nil
		}
		// The core's targets, read again, list it once whatever read was
		// asked before
		m.failed = ""
		m.savedID = msg.Target.ID
		m.state = StateSelectTarget
		reload := m.ReloadTargets()
		return m, reload

	case DirectoriesLoadedMsg:
		if !m.listing.Take(msg.Request) {
			return m, nil
		}
		if msg.Err != nil {
			m.failed = errmsg.FormatWith(errmsg.OpFolderList, msg.Path, msg.Err)
			return m, nil
		}
		m.failed = ""
		m.currentPath = msg.Path
		m.directories = msg.Dirs
		m.dirIdx = 0
		return m, nil
	}

	return m, nil
}

// handleKey handles a key on the screen shown: a failure said on a screen
// goes with it.
func (m *Model) handleKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	shown := m.state
	p, cmd := m.handleScreenKey(msg)
	if m.state != shown {
		m.failed = ""
	}
	return p, cmd
}

func (m *Model) handleScreenKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
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

		// Whether its device is there is for the core to say: it looks
		// again, the volumes read when the popup opened may be stale.
		return m, func() tea.Msg {
			return ActionMsg(StartExport{
				Target:      target,
				Tracks:      m.tracks,
				ConvertFLAC: m.convertFLAC,
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
		m.newTarget = core.ExportTarget{
			DeviceUUID:      vol.UUID,
			DeviceLabel:     vol.Label,
			Name:            name,
			Subfolder:       "/",
			FolderStructure: core.ExportFlat,
		}
		// Initialize directory browser
		m.mountPath = vol.MountPath
		m.currentPath = "/"
		m.directories = nil
		m.dirIdx = 0
		m.state = StateNewTargetFolder
		return m, m.listing.Ask(listDirectories(vol.MountPath, "/"))
	}

	return m, nil
}

//nolint:goconst // Key strings are clearer inline
func (m *Model) handleNewTargetFolderKey(msg tea.KeyMsg) (popup.Popup, tea.Cmd) {
	// Calculate max index: directories + ".." if not at root
	hasParent := m.currentPath != "/"
	maxIdx := len(m.directories)
	if hasParent {
		maxIdx++ // Account for ".." entry at index 0
	}

	switch msg.String() {
	case "esc":
		m.listing.Forget()
		m.state = StateNewTarget
		return m, nil

	case "up", "k":
		if m.dirIdx > 0 {
			m.dirIdx--
		}

	case "down", "j":
		if m.dirIdx < maxIdx-1 {
			m.dirIdx++
		}

	case "enter":
		// Navigate into selected directory
		if maxIdx == 0 {
			return m, nil
		}

		if hasParent && m.dirIdx == 0 {
			// Selected ".." - go up one level
			return m.navigateUp()
		}

		// Get the actual directory index
		dirIndex := m.dirIdx
		if hasParent {
			dirIndex-- // Adjust for ".." entry
		}

		if dirIndex < 0 || dirIndex >= len(m.directories) {
			return m, nil
		}

		// Navigate into the selected directory
		selectedDir := m.directories[dirIndex]
		newPath := filepath.Join(m.currentPath, selectedDir)
		return m, m.listing.Ask(listDirectories(m.mountPath, newPath))

	case "backspace":
		// Go up one level if not at root
		if m.currentPath != "/" {
			return m.navigateUp()
		}

	case " ":
		// Select current directory as target: a listing still asked is
		// for the browser left
		m.listing.Forget()
		m.newTarget.Subfolder = m.currentPath
		m.structureIdx = 0
		m.folderStructure = core.ExportFlat
		m.state = StateNewTargetConfig
	}

	return m, nil
}

// navigateUp goes to the parent directory.
func (m *Model) navigateUp() (popup.Popup, tea.Cmd) {
	parent := filepath.Dir(m.currentPath)
	if parent == "." {
		parent = "/"
	}
	return m, m.listing.Ask(listDirectories(m.mountPath, parent))
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
		m.folderStructure = core.ExportFlat

	case "2":
		m.structureIdx = 1
		m.folderStructure = core.ExportHierarchical

	case "3":
		m.structureIdx = 2
		m.folderStructure = core.ExportSingle

	case "enter":
		m.newTarget.FolderStructure = m.folderStructure
		return m, m.saving.Ask(createTarget(m.exports, m.newTarget))
	}

	return m, nil
}

func (m *Model) updateFolderStructure() {
	switch m.structureIdx {
	case 0:
		m.folderStructure = core.ExportFlat
	case 1:
		m.folderStructure = core.ExportHierarchical
	case 2:
		m.folderStructure = core.ExportSingle
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
			_, size := utf8.DecodeLastRuneInString(m.renameInput)
			m.renameInput = m.renameInput[:len(m.renameInput)-size]
		}

	default:
		// Append all non-control runes. Iterating over Runes (rather than
		// requiring exactly one) also handles multi-rune paste events.
		for _, r := range msg.Runes {
			if !unicode.IsControl(r) {
				m.renameInput += string(r)
			}
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
		m.newTarget = core.ExportTarget{
			DeviceUUID:      "", // Empty = custom folder target
			DeviceLabel:     "",
			Name:            name,
			Subfolder:       expandedPath,
			FolderStructure: core.ExportFlat,
		}
		m.structureIdx = 0
		m.folderStructure = core.ExportFlat
		m.state = StateCustomFolderConfig
		return m, nil

	case "backspace":
		if m.customFolderInput != "" {
			_, size := utf8.DecodeLastRuneInString(m.customFolderInput)
			m.customFolderInput = m.customFolderInput[:len(m.customFolderInput)-size]
		}

	default:
		// Append all non-control runes. Iterating over Runes (rather than
		// requiring exactly one) also handles multi-rune paste events.
		for _, r := range msg.Runes {
			if !unicode.IsControl(r) {
				m.customFolderInput += string(r)
			}
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
		m.folderStructure = core.ExportFlat

	case "2":
		m.structureIdx = 1
		m.folderStructure = core.ExportHierarchical

	case "3":
		m.structureIdx = 2
		m.folderStructure = core.ExportSingle

	case "enter":
		m.newTarget.FolderStructure = m.folderStructure
		return m, m.saving.Ask(createTarget(m.exports, m.newTarget))
	}

	return m, nil
}
