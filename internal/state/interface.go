// internal/state/interface.go
package state

import "database/sql"

// Interface defines the state manager contract for dependency injection and testing.
type Interface interface {
	DB() *sql.DB
	SaveNavigation(state NavigationState)
	GetNavigation() (*NavigationState, error)
	SaveQueue(state QueueState) error
	GetQueue() (*QueueState, error)
	Close() error
}

// Verify Manager implements Interface at compile time.
var _ Interface = (*Manager)(nil)
