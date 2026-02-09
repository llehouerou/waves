// Package similarartists provides a popup for discovering similar artists.
package similarartists

import "github.com/llehouerou/waves/internal/ui/action"

// Action types for communication with root app.

// Close requests closing the popup.
type Close struct{}

func (Close) ActionType() string { return "similarartists.Close" }

// GoToArtist requests navigating to an artist in the library.
type GoToArtist struct {
	Name string
}

func (GoToArtist) ActionType() string { return "similarartists.GoToArtist" }

// OpenDownload requests opening the download popup for an artist.
type OpenDownload struct {
	Name string
}

func (OpenDownload) ActionType() string { return "similarartists.OpenDownload" }

// ActionMsg wraps an action with the component source.
func ActionMsg(a action.Action) action.Msg {
	return action.Msg{Source: "similarartists", Action: a}
}
