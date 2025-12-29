// internal/playback/state.go
package playback

// State represents the playback state.
type State int

const (
	StateStopped State = iota
	StatePlaying
	StatePaused
)

// String returns the state name.
func (s State) String() string {
	switch s {
	case StateStopped:
		return "Stopped"
	case StatePlaying:
		return "Playing"
	case StatePaused:
		return "Paused"
	default:
		return "Unknown"
	}
}

// IsActive returns true if playback is active (playing or paused).
func (s State) IsActive() bool {
	return s == StatePlaying || s == StatePaused
}

// RepeatMode defines the repeat behavior.
type RepeatMode int

const (
	RepeatOff RepeatMode = iota
	RepeatAll
	RepeatOne
	RepeatRadio
)

// String returns the repeat mode name.
func (m RepeatMode) String() string {
	switch m {
	case RepeatOff:
		return "Off"
	case RepeatAll:
		return "All"
	case RepeatOne:
		return "One"
	case RepeatRadio:
		return "Radio"
	default:
		return "Unknown"
	}
}
