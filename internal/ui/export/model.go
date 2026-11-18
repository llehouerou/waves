// Package export provides the export popup UI.
package export

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/request"
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
	listed      bool // the targets were listed once: a list read again keeps the cursor
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

	// Failures, said to the user
	targetsFailed string // why the targets could not be read
	volumesFailed string // why the devices could not be read
	failed        string // what failed on the screen shown: said until the screen changes or a later answer is taken

	// Dependencies
	exports Exports

	// What the popup waits for: each answer is taken once, from the last
	// request asked, and dropped otherwise
	volumesRead request.Pending // the volumes, asked by Init
	targetsRead request.Pending // the targets, asked by Init, ReloadTargets and after a save
	listing     request.Pending // the folders of the device browsed
	saving      request.Pending // the new target
	savedID     int64           // the target saved, selected once the targets read again come
}

// New creates a new export popup.
func New(exports Exports) *Model {
	return &Model{
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
func (m *Model) HasFLAC() bool {
	return m.flacCount > 0
}

// SelectedTarget returns the currently selected target, if any.
func (m *Model) SelectedTarget() (core.ExportTarget, bool) {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.targets) {
		return core.ExportTarget{}, false
	}
	return m.targets[m.selectedIdx], true
}

// TrackCount returns the number of tracks to export.
func (m *Model) TrackCount() int {
	return len(m.tracks)
}

// isTargetConnected reports whether a target could be exported to when the
// popup read the volumes: a hint for the list, the core decides at the start.
func (m *Model) isTargetConnected(target core.ExportTarget) bool {
	// A folder of this machine: the export creates it.
	if target.DeviceUUID == "" {
		return target.Subfolder != ""
	}
	return slices.ContainsFunc(m.volumes, func(vol core.ExportVolume) bool {
		return vol.UUID == target.DeviceUUID
	})
}

// ReloadTargets reads the targets again, after the app renamed or deleted
// one: the read asked before, if it comes later, is dropped.
func (m *Model) ReloadTargets() tea.Cmd {
	return m.targetsRead.Ask(loadTargets(m.exports))
}

// keepCursor puts the cursor, on a list of targets read again, where it was
// before: on "New target", or on the same target wherever it is now, or on
// its row when it is gone, the last row when that row is gone too.
func (m *Model) keepCursor(was core.ExportTarget, onTarget bool) {
	switch {
	case !onTarget:
		m.selectedIdx = len(m.targets)
	case !m.selectTarget(was.ID):
		m.selectedIdx = max(min(m.selectedIdx, len(m.targets)-1), 0)
	}
}

// selectTarget selects the target id, if it is listed, and reports whether
// it is.
func (m *Model) selectTarget(id int64) bool {
	i := slices.IndexFunc(m.targets, func(t core.ExportTarget) bool { return t.ID == id })
	if i >= 0 {
		m.selectedIdx = i
	}
	return i >= 0
}

// autoSelectTarget selects a target if its device is connected: the cursor
// the popup opens with.
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
