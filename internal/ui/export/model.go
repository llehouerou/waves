// Package export provides the export popup UI.
package export

import (
	"github.com/llehouerou/waves/internal/export"
)

// State represents the popup state.
type State int

const (
	StateSelectTarget State = iota
	StateNewTarget
	StateNewTargetFolder
	StateNewTargetConfig
	StateRenameTarget
	StateCustomFolder
	StateCustomFolderConfig
)

// Model is the export popup model.
type Model struct {
	state  State
	width  int
	height int

	// What we're exporting
	tracks    []export.Track
	flacCount int
	albumName string // For display

	// Target selection
	targets     []export.Target
	volumes     []export.Volume
	selectedIdx int
	convertFLAC bool

	// New target wizard
	newTarget       export.Target
	volumeIdx       int
	structureIdx    int
	folderStructure export.FolderStructure

	// Rename target
	renameInput    string
	renameTargetID int64

	// Custom folder target
	customFolderInput string

	// Dependencies
	repo *export.TargetRepository
}

// New creates a new export popup.
func New(repo *export.TargetRepository) Model {
	return Model{
		state:           StateSelectTarget,
		repo:            repo,
		folderStructure: export.FolderStructureFlat,
	}
}

// SetTracks sets the tracks to export.
func (m *Model) SetTracks(tracks []export.Track, albumName string) {
	m.tracks = tracks
	m.albumName = albumName

	// Count FLAC files
	m.flacCount = 0
	for _, t := range tracks {
		if export.NeedsConversion(t.Extension) {
			m.flacCount++
		}
	}
}

// SetSize sets the popup dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// HasFLAC returns true if there are FLAC files to convert.
func (m Model) HasFLAC() bool {
	return m.flacCount > 0
}

// SelectedTarget returns the currently selected target, if any.
func (m Model) SelectedTarget() (export.Target, bool) {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.targets) {
		return export.Target{}, false
	}
	return m.targets[m.selectedIdx], true
}

// TrackCount returns the number of tracks to export.
func (m Model) TrackCount() int {
	return len(m.tracks)
}

// findMountPath returns the mount path for a device UUID.
func (m Model) findMountPath(uuid string) string {
	for _, vol := range m.volumes {
		if vol.UUID == uuid {
			return vol.MountPath
		}
	}
	return ""
}

// isTargetConnected checks if a target's device is mounted or folder exists.
func (m Model) isTargetConnected(target export.Target) bool {
	// Custom folder targets - check if path exists
	if target.DeviceUUID == "" {
		return m.folderExists(target.Subfolder)
	}
	// Device-based targets - check if mounted
	return m.findMountPath(target.DeviceUUID) != ""
}

// folderExists checks if a folder path exists.
func (m Model) folderExists(path string) bool {
	// We'll check this via the Subfolder field for custom targets
	// For now, assume it exists - actual check happens at export time
	return path != ""
}

// isCustomFolderTarget returns true if the target is a custom folder (no device).
func (m Model) isCustomFolderTarget(target export.Target) bool {
	return target.DeviceUUID == ""
}

// autoSelectTarget selects a target if its device is connected.
func (m *Model) autoSelectTarget() {
	for i, target := range m.targets {
		for _, vol := range m.volumes {
			if vol.UUID == target.DeviceUUID {
				m.selectedIdx = i
				return
			}
		}
	}
}
