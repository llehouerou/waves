// internal/app/layout_manager.go
package app

import (
	"github.com/llehouerou/waves/internal/ui/playerbar"
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

// NarrowThreshold is the terminal width below which the layout switches to narrow mode.
// In narrow mode, the queue panel is displayed below the navigator instead of beside it.
const NarrowThreshold = 120

// LayoutManager manages window dimensions, queue visibility, and the queue panel.
type LayoutManager struct {
	width             int
	height            int
	queueVisible      bool
	queuePanel        queuepanel.Model
	playerDisplayMode playerbar.DisplayMode
}

// NewLayoutManager creates a new LayoutManager with the given queue panel.
func NewLayoutManager(queuePanel queuepanel.Model) LayoutManager {
	return LayoutManager{
		queueVisible:      true,
		queuePanel:        queuePanel,
		playerDisplayMode: playerbar.ModeExpanded,
	}
}

// --- Dimensions ---

// SetSize updates the window dimensions.
func (l *LayoutManager) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// Width returns the window width.
func (l *LayoutManager) Width() int {
	return l.width
}

// Height returns the window height.
func (l *LayoutManager) Height() int {
	return l.height
}

// Dimensions returns both width and height.
func (l *LayoutManager) Dimensions() (width, height int) {
	return l.width, l.height
}

// IsNarrowMode returns true if the terminal width is below the narrow threshold.
// In narrow mode, the queue panel is stacked below the navigator.
func (l *LayoutManager) IsNarrowMode() bool {
	return l.width < NarrowThreshold
}

// --- Queue Visibility ---

// ToggleQueue toggles queue panel visibility.
func (l *LayoutManager) ToggleQueue() {
	l.queueVisible = !l.queueVisible
}

// ShowQueue makes the queue panel visible.
func (l *LayoutManager) ShowQueue() {
	l.queueVisible = true
}

// HideQueue hides the queue panel.
func (l *LayoutManager) HideQueue() {
	l.queueVisible = false
}

// IsQueueVisible returns true if the queue panel is visible.
func (l *LayoutManager) IsQueueVisible() bool {
	return l.queueVisible
}

// --- Queue Panel ---

// QueuePanel returns a pointer to the queue panel model.
func (l *LayoutManager) QueuePanel() *queuepanel.Model {
	return &l.queuePanel
}

// SetQueuePanel replaces the queue panel model.
func (l *LayoutManager) SetQueuePanel(panel queuepanel.Model) {
	l.queuePanel = panel
}

// --- Player Display Mode ---

// PlayerDisplayMode returns the current player bar display mode.
func (l *LayoutManager) PlayerDisplayMode() playerbar.DisplayMode {
	return l.playerDisplayMode
}

// SetPlayerDisplayMode sets the player bar display mode.
func (l *LayoutManager) SetPlayerDisplayMode(mode playerbar.DisplayMode) {
	l.playerDisplayMode = mode
}

// --- Layout Calculations ---

// NavigatorWidth returns the available width for navigators.
// In narrow mode or when queue is hidden, returns full width.
func (l *LayoutManager) NavigatorWidth() int {
	if l.queueVisible && !l.IsNarrowMode() {
		return l.width * 2 / 3
	}
	return l.width
}

// QueueWidth returns the width for the queue panel.
// In narrow mode, returns full width since queue is stacked below navigator.
func (l *LayoutManager) QueueWidth() int {
	if l.IsNarrowMode() {
		return l.width
	}
	return l.width - l.NavigatorWidth()
}

// ResizeQueuePanel updates the queue panel dimensions based on current layout.
// Always updates the size, even when hidden, so it's correct when shown.
func (l *LayoutManager) ResizeQueuePanel(height int) {
	l.queuePanel.SetSize(l.QueueWidth(), height)
}

// --- View Rendering ---

// RenderQueuePanel renders the queue panel.
func (l *LayoutManager) RenderQueuePanel() string {
	return l.queuePanel.View()
}
