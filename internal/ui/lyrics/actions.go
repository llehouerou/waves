package lyrics

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/core"
	"github.com/llehouerou/waves/internal/ui/action"
)

// Close signals the lyrics popup should close.
type Close struct{}

// ActionType implements action.Action.
func (a Close) ActionType() string { return "lyrics.close" }

// Passthrough signals a key should be passed to the main handler.
type Passthrough struct {
	Key tea.KeyMsg
}

// ActionType implements action.Action.
func (a Passthrough) ActionType() string { return "lyrics.passthrough" }

// FetchedMsg is sent when lyrics have been fetched: the lyrics of the track at
// TrackPath, or why there are none (core.ErrNoLyrics when nobody has them).
type FetchedMsg struct {
	TrackPath string
	Lyrics    *core.TrackLyrics
	Err       error
}

// ActionType implements action.Action.
func (a FetchedMsg) ActionType() string { return "lyrics.fetched" }

// Verify interfaces at compile time.
var (
	_ action.Action = Close{}
	_ action.Action = Passthrough{}
	_ action.Action = FetchedMsg{}
)
