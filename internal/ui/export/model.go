// Package export provides the export popup UI.
package export

import (
	"slices"

	"github.com/llehouerou/waves/internal/core"
)

// Exports is what the popup asks of the core: the saved targets, the
// removable devices mounted now, and saving a new target. Starting an export,
// renaming and deleting a target are actions the app carries out.
type Exports interface {
	Targets() ([]core.ExportTarget, error)
	Volumes() ([]core.ExportVolume, error)
	CreateTarget(t core.ExportTarget) (id int64, err error)
}

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
	tracks    []core.ExportTrack
	flacCount int
	albumName string // For display

	// Target selection
	targets     []core.ExportTarget
	volumes     []core.ExportVolume
	selectedIdx int
	convertFLAC bool

	// New target wizard
	newTarget       core.ExportTarget
	volumeIdx       int
	structureIdx    int
	folderStructure core.ExportFolderStructure

	// Directory browser for USB targets
	mountPath   string   // Base mount path of selected device
	currentPath string   // Current browsing path relative to mount root
	directories []string // Subdirectory names in current path
	dirIdx      int      // Currently highlighted directory index

	// Rename target
	renameInput    string
	renameTargetID int64

	// Custom folder target
	customFolderInput string

	// Dependencies
	exports Exports
}

// New creates a new export popup.
func New(exports Exports) Model {
	return Model{
		state:           StateSelectTarget,
		exports:         exports,
		folderStructure: core.ExportFlat,
	}
}

// SetTracks sets the tracks to export.
func (m *Model) SetTracks(tracks []core.ExportTrack, albumName string) {
	m.tracks = tracks
	m.albumName = albumName

	// Count FLAC files
	m.flacCount = 0
	for i := range tracks {
		if core.ExportCanConvert(tracks[i].Extension) {
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
func (m Model) SelectedTarget() (core.ExportTarget, bool) {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.targets) {
		return core.ExportTarget{}, false
	}
	return m.targets[m.selectedIdx], true
}

// TrackCount returns the number of tracks to export.
func (m Model) TrackCount() int {
	return len(m.tracks)
}

// isTargetConnected reports whether a target could be exported to when the
// popup read the volumes: a hint for the list, the core decides at the start.
func (m Model) isTargetConnected(target core.ExportTarget) bool {
	// A folder of this machine: the export creates it.
	if target.DeviceUUID == "" {
		return target.Subfolder != ""
	}
	return slices.ContainsFunc(m.volumes, func(vol core.ExportVolume) bool {
		return vol.UUID == target.DeviceUUID
	})
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
