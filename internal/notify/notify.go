// Package notify provides desktop notifications via D-Bus.
package notify

// Urgency represents notification priority levels per freedesktop spec.
type Urgency byte

const (
	UrgencyLow      Urgency = 0
	UrgencyNormal   Urgency = 1
	UrgencyCritical Urgency = 2
)

// Notification contains data for a desktop notification.
type Notification struct {
	Title      string  // Summary text (required)
	Body       string  // Body text (optional, supports basic markup)
	Icon       string  // Path to image file or icon name (optional)
	Timeout    int32   // ms, -1 = server default, 0 = never expire
	ReplacesID uint32  // 0 = new notification, >0 = replace existing
	Urgency    Urgency // Low, Normal, Critical
}

// Notifier sends desktop notifications.
type Notifier interface {
	// Notify sends a notification and returns its ID.
	// Returns 0 and nil error if notifications are disabled or unavailable.
	Notify(n Notification) (uint32, error)
	// Close closes a notification by ID.
	Close(id uint32) error
}
