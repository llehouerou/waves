package export

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/export"
	"github.com/llehouerou/waves/internal/ui/action"
)

// Source is the action source identifier for the export popup.
const Source = "export"

// ActionMsg wraps an action with the source identifier.
func ActionMsg(a action.Action) tea.Msg {
	return action.Msg{
		Source: Source,
		Action: a,
	}
}

// Close signals that the popup should be closed.
type Close struct{}

func (Close) ActionType() string { return "export.Close" }

// StartExport signals that the export should begin.
type StartExport struct {
	Target      export.Target
	Tracks      []export.Track
	ConvertFLAC bool
	MountPath   string
}

func (StartExport) ActionType() string { return "export.StartExport" }

// DeviceNotConnected signals that the target device is not mounted.
type DeviceNotConnected struct {
	TargetName string
}

func (DeviceNotConnected) ActionType() string { return "export.DeviceNotConnected" }

// DeleteTarget signals that a target should be deleted.
type DeleteTarget struct {
	ID   int64
	Name string
}

func (DeleteTarget) ActionType() string { return "export.DeleteTarget" }

// RenameTarget signals that a target should be renamed.
type RenameTarget struct {
	ID      int64
	NewName string
}

func (RenameTarget) ActionType() string { return "export.RenameTarget" }
