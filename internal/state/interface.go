// internal/state/interface.go
package state

import (
	"database/sql"

	"github.com/llehouerou/waves/internal/ui/albumview"
)

// Interface defines the state manager contract for dependency injection and testing.
type Interface interface {
	DB() *sql.DB
	SaveNavigation(state NavigationState)
	GetNavigation() (*NavigationState, error)
	SaveQueue(state QueueState) error
	GetQueue() (*QueueState, error)
	ListAlbumPresets() ([]albumview.Preset, error)
	SaveAlbumPreset(name string, settings albumview.Settings) (int64, error)
	DeleteAlbumPreset(id int64) error
	Close() error
}

// Verify Manager implements Interface at compile time.
var _ Interface = (*Manager)(nil)
