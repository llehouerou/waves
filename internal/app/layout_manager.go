// internal/app/layout_manager.go
package app

import (
	"github.com/llehouerou/waves/internal/ui/queuepanel"
)

// LayoutManager manages window dimensions, queue visibility, and the queue panel.
type LayoutManager struct {
	width        int
	height       int
	queueVisible bool
	queuePanel   queuepanel.Model
}

// NewLayoutManager creates a new LayoutManager with the given queue panel.
func NewLayoutManager(queuePanel queuepanel.Model) LayoutManager {
	return LayoutManager{
		queueVisible: true,
		queuePanel:   queuePanel,
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

// --- Layout Calculations ---

// NavigatorWidth returns the available width for navigators.
func (l *LayoutManager) NavigatorWidth() int {
	if l.queueVisible {
		return l.width * 2 / 3
	}
	return l.width
}

// QueueWidth returns the width for the queue panel.
func (l *LayoutManager) QueueWidth() int {
	return l.width - l.NavigatorWidth()
}

// ResizeQueuePanel updates the queue panel dimensions based on current layout.
func (l *LayoutManager) ResizeQueuePanel(height int) {
	if l.queueVisible {
		l.queuePanel.SetSize(l.QueueWidth(), height)
	}
}
