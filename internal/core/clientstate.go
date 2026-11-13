package core

import (
	"github.com/llehouerou/waves/internal/albumpreset"
	"github.com/llehouerou/waves/internal/state"
)

// NavigationState is where the user was in a client: its view, and what was
// selected in each.
type NavigationState = state.NavigationState

// clientStateAPI is the part of the state database a client calls as it is.
type clientStateAPI interface {
	// GetNavigation returns nil on a first run: nothing was saved yet.
	GetNavigation() (*NavigationState, error)
	// SaveNavigation returns at once: the state is written a moment later,
	// and at Close at the latest.
	SaveNavigation(nav NavigationState)

	ListAlbumPresets() ([]albumpreset.Preset, error)
	SaveAlbumPreset(name string, settings albumpreset.Settings) (int64, error)
	DeleteAlbumPreset(id int64) error
}

// ClientState is what the core keeps for a client between two runs: where
// the user was, and the album presets. The core stores it and does not read
// it: navigation is the client's.
type ClientState struct {
	clientStateAPI
}

// ClientState returns what the core keeps for a client between two runs.
func (c *Core) ClientState() *ClientState {
	return &ClientState{clientStateAPI: c.state}
}
