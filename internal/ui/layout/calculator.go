// Package layout provides pure functions for UI dimension calculations.
package layout

// NarrowThreshold is the terminal width below which the layout switches to narrow mode.
// In narrow mode, the queue panel is displayed below the navigator instead of beside it.
const NarrowThreshold = 120

// NotificationBorderHeight is the height of borders around notifications.
const NotificationBorderHeight = 2

// ContentOpts contains the parameters needed to calculate content height.
type ContentOpts struct {
	HeaderHeight      int
	PlayerBarHeight   int // 0 if player is stopped
	JobBarHeight      int // 0 if no active jobs
	NotificationCount int
}

// ContentHeight calculates the available height for the main content area
// (navigator + queue). This is the terminal height minus header, player bar,
// job bar, and notifications.
func ContentHeight(windowHeight int, opts ContentOpts) int {
	height := windowHeight
	height -= opts.HeaderHeight
	height -= opts.PlayerBarHeight
	height -= opts.JobBarHeight
	height -= NotificationHeight(opts.NotificationCount)
	return height
}

// NotificationHeight returns the height needed for the given number of notifications.
func NotificationHeight(count int) int {
	if count == 0 {
		return 0
	}
	return count + NotificationBorderHeight
}

// NavigatorHeight calculates the available height for navigators.
// In narrow mode with queue visible, returns 2/3 of content height.
// Otherwise returns full content height.
func NavigatorHeight(contentHeight int, narrowMode, queueVisible bool) int {
	if narrowMode && queueVisible {
		return contentHeight * 2 / 3
	}
	return contentHeight
}

// QueueHeight calculates the available height for the queue panel.
// In narrow mode, returns 1/3 of content height (stacked below navigator).
// Otherwise returns same as navigator height (side by side layout).
func QueueHeight(contentHeight int, narrowMode, queueVisible bool) int {
	if narrowMode {
		navHeight := NavigatorHeight(contentHeight, narrowMode, queueVisible)
		return contentHeight - navHeight
	}
	return NavigatorHeight(contentHeight, narrowMode, queueVisible)
}

// IsNarrowMode returns true if the terminal width is below the narrow threshold.
func IsNarrowMode(width int) bool {
	return width < NarrowThreshold
}

// NavigatorWidth calculates the available width for navigators.
// In narrow mode or when queue is hidden, returns full width.
// Otherwise returns 2/3 of the width.
func NavigatorWidth(windowWidth int, narrowMode, queueVisible bool) int {
	if queueVisible && !narrowMode {
		return windowWidth * 2 / 3
	}
	return windowWidth
}

// QueueWidth calculates the width for the queue panel.
// In narrow mode, returns full width since queue is stacked below navigator.
// Otherwise returns the remaining width after navigator.
func QueueWidth(windowWidth int, narrowMode, queueVisible bool) int {
	if narrowMode {
		return windowWidth
	}
	return windowWidth - NavigatorWidth(windowWidth, narrowMode, queueVisible)
}

// PlayerBarRow calculates the 1-based row number where the player bar starts.
// Returns 0 if playerBarHeight is 0 (player is stopped).
func PlayerBarRow(windowHeight, playerBarHeight, jobBarHeight, notificationCount int) int {
	if playerBarHeight == 0 {
		return 0
	}

	row := windowHeight
	row -= NotificationHeight(notificationCount)
	row -= jobBarHeight
	row -= playerBarHeight

	// Convert to 1-based row number
	return row + 1
}
