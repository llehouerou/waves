package export

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/export"
)

// VolumesLoadedMsg contains detected volumes.
type VolumesLoadedMsg struct {
	Volumes []export.Volume
	Err     error
}

// TargetsLoadedMsg contains saved targets.
type TargetsLoadedMsg struct {
	Targets []export.Target
	Err     error
}

// TargetCreatedMsg signals a new target was created.
type TargetCreatedMsg struct {
	Target export.Target
	Err    error
}

// LoadVolumesCmd detects mounted volumes.
func LoadVolumesCmd() tea.Cmd {
	return func() tea.Msg {
		volumes, err := export.DetectVolumes()
		return VolumesLoadedMsg{Volumes: volumes, Err: err}
	}
}

// LoadTargetsCmd loads saved targets.
func LoadTargetsCmd(repo *export.TargetRepository) tea.Cmd {
	return func() tea.Msg {
		targets, err := repo.List()
		return TargetsLoadedMsg{Targets: targets, Err: err}
	}
}

// CreateTargetCmd saves a new target.
func CreateTargetCmd(repo *export.TargetRepository, target export.Target) tea.Cmd {
	return func() tea.Msg {
		id, err := repo.Create(target)
		if err == nil {
			target.ID = id
		}
		return TargetCreatedMsg{Target: target, Err: err}
	}
}
