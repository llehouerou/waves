//go:build !linux

package notify

// stubNotifier is a no-op notifier for non-Linux platforms.
type stubNotifier struct{}

// New returns a no-op notifier on non-Linux platforms.
func New() (Notifier, error) {
	return &stubNotifier{}, nil
}

func (s *stubNotifier) Notify(_ Notification) (uint32, error) {
	return 0, nil
}

func (s *stubNotifier) Close(_ uint32) error {
	return nil
}
