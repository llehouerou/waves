// internal/player/state.go
package player

// State represents the playback state machine.
//
// The state machine has three states with the following valid transitions:
//
//	┌──────────┐      play       ┌──────────┐
//	│  Stopped │ ───────────────▶│  Playing │
//	└──────────┘                 └──────────┘
//	     ▲                            │ │
//	     │ stop                 pause │ │ stop
//	     │                            ▼ │
//	     │                       ┌──────────┐
//	     └───────────────────────│  Paused  │
//	                  stop       └──────────┘
//	                                  │
//	                           resume │
//	                                  │
//	                                  ▼
//	                             Playing
//
// Valid transitions:
//   - Stopped → Playing (via Play)
//   - Playing → Paused  (via Pause)
//   - Playing → Stopped (via Stop)
//   - Paused  → Playing (via Resume)
//   - Paused  → Stopped (via Stop)
//
// Toggle() cycles: Playing ↔ Paused (no-op if Stopped)
//
// Invalid/No-op transitions (handled gracefully):
//   - Stopped → Paused  (ignored)
//   - Stopped → Stopped (ignored)
//   - Paused  → Paused  (ignored)
//   - Playing → Playing (ignored, Play() stops first)

// String returns the state name for debugging.
func (s State) String() string {
	switch s {
	case Stopped:
		return "Stopped"
	case Playing:
		return "Playing"
	case Paused:
		return "Paused"
	default:
		return "Unknown"
	}
}

// IsActive returns true if playback is active (Playing or Paused).
func (s State) IsActive() bool {
	return s == Playing || s == Paused
}

// CanPause returns true if the state allows pausing.
func (s State) CanPause() bool {
	return s == Playing
}

// CanResume returns true if the state allows resuming.
func (s State) CanResume() bool {
	return s == Paused
}
