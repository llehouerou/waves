package notify

import "testing"

func TestUrgencyValues(t *testing.T) {
	// Verify urgency constants match D-Bus spec
	if UrgencyLow != 0 {
		t.Errorf("UrgencyLow = %d, want 0", UrgencyLow)
	}
	if UrgencyNormal != 1 {
		t.Errorf("UrgencyNormal = %d, want 1", UrgencyNormal)
	}
	if UrgencyCritical != 2 {
		t.Errorf("UrgencyCritical = %d, want 2", UrgencyCritical)
	}
}

func TestNotificationZeroValue(t *testing.T) {
	var n Notification
	if n.Urgency != UrgencyLow {
		t.Errorf("zero value Urgency = %d, want UrgencyLow (0)", n.Urgency)
	}
	if n.Timeout != 0 {
		t.Error("zero value Timeout should be 0 (never expire)")
	}
	if n.ReplacesID != 0 {
		t.Error("zero value ReplacesID should be 0 (new notification)")
	}
}
