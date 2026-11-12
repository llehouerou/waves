// internal/state/interface.go
package state

import (
	"database/sql"
	"time"

	"github.com/llehouerou/waves/internal/albumpreset"
)

// Interface defines the state manager contract for dependency injection and testing.
type Interface interface {
	DB() *sql.DB
	SaveNavigation(state NavigationState)
	GetNavigation() (*NavigationState, error)
	SaveQueue(state QueueState) error
	GetQueue() (*QueueState, error)
	GetVolume() (*VolumeState, error)
	SaveVolume(volume float64, muted bool) error
	GetLastfmSession() (*LastfmSession, error)
	SaveLastfmSession(username, sessionKey string) error
	DeleteLastfmSession() error
	AddPendingScrobble(s PendingScrobble) error
	GetPendingScrobbles() ([]PendingScrobble, error)
	DeletePendingScrobble(id int64) error
	UpdatePendingScrobbleAttempt(id int64, errMsg string) error
	DeleteOldPendingScrobbles(maxAge time.Duration) error
	ListAlbumPresets() ([]albumpreset.Preset, error)
	SaveAlbumPreset(name string, settings albumpreset.Settings) (int64, error)
	DeleteAlbumPreset(id int64) error
	Close() error
}

// Verify Manager implements Interface at compile time.
var _ Interface = (*Manager)(nil)
