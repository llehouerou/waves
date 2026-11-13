package core

import (
	"io"

	"github.com/llehouerou/waves/internal/mpris"
	"github.com/llehouerou/waves/internal/playback"
)

// serveMPRIS registers waves on the session bus as a media player over the
// playback service. MPRIS listens to the service itself rather than to the
// core's loop: each track it announces makes it look for a cover on the disk.
func serveMPRIS(svc playback.Service) io.Closer {
	// It fails without a session bus: waves then plays without MPRIS.
	adapter, err := mpris.New(svc)
	if err != nil {
		return nil
	}
	return adapter
}
