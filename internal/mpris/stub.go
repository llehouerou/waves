//go:build !linux

package mpris

import "github.com/llehouerou/waves/internal/playback"

// Adapter is a no-op on non-Linux platforms.
type Adapter struct{}

// New returns a no-op adapter on non-Linux platforms.
func New(_ playback.Service) (*Adapter, error) {
	return &Adapter{}, nil
}

// Resubscribe is a no-op on non-Linux platforms.
func (a *Adapter) Resubscribe(_ playback.Service) {}

// Close is a no-op on non-Linux platforms.
func (a *Adapter) Close() error {
	return nil
}
