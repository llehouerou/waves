package albumview

import (
	"github.com/llehouerou/waves/internal/ui/action"
)

// Grouping popup actions

// GroupingApplied is emitted when grouping settings are confirmed.
type GroupingApplied struct {
	Fields    []GroupField
	SortOrder SortOrder
	DateField DateFieldType
}

// ActionType implements action.Action.
func (a GroupingApplied) ActionType() string { return "albumview.grouping_applied" }

// GroupingCanceled is emitted when grouping popup is canceled.
type GroupingCanceled struct{}

// ActionType implements action.Action.
func (a GroupingCanceled) ActionType() string { return "albumview.grouping_canceled" }

// GroupingActionMsg creates an action.Msg for a grouping action.
func GroupingActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "albumview.grouping", Action: a}
}

// Sorting popup actions

// SortingApplied is emitted when sorting settings are confirmed.
type SortingApplied struct {
	Criteria []SortCriterion
}

// ActionType implements action.Action.
func (a SortingApplied) ActionType() string { return "albumview.sorting_applied" }

// SortingCanceled is emitted when sorting popup is canceled.
type SortingCanceled struct{}

// ActionType implements action.Action.
func (a SortingCanceled) ActionType() string { return "albumview.sorting_canceled" }

// SortingActionMsg creates an action.Msg for a sorting action.
func SortingActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "albumview.sorting", Action: a}
}

// Presets popup actions

// PresetLoaded is emitted when a preset is selected.
type PresetLoaded struct {
	Settings Settings
	PresetID int64
}

// ActionType implements action.Action.
func (a PresetLoaded) ActionType() string { return "albumview.preset_loaded" }

// PresetSaved is emitted when a preset is saved.
type PresetSaved struct {
	Name     string
	Settings Settings
}

// ActionType implements action.Action.
func (a PresetSaved) ActionType() string { return "albumview.preset_saved" }

// PresetDeleted is emitted when a preset is deleted.
type PresetDeleted struct {
	ID int64
}

// ActionType implements action.Action.
func (a PresetDeleted) ActionType() string { return "albumview.preset_deleted" }

// PresetsClosed is emitted when the presets popup is closed without action.
type PresetsClosed struct{}

// ActionType implements action.Action.
func (a PresetsClosed) ActionType() string { return "albumview.presets_closed" }

// PresetsActionMsg creates an action.Msg for a presets action.
func PresetsActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "albumview.presets", Action: a}
}
