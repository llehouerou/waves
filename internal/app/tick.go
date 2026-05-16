// internal/app/tick.go
package app

import tea "github.com/charmbracelet/bubbletea"

// ensureTickRunning starts a single 1s tick chain if none is alive.
// It returns nil when a chain is already running, which is what prevents the
// per-track accumulation in issue #28. The returned command, if any, must be
// scheduled by the caller; the mutated Model must be returned (MVU).
func (m *Model) ensureTickRunning() tea.Cmd {
	if m.tickRunning {
		return nil
	}
	m.tickRunning = true
	m.tickGen++
	return TickCmd(m.tickGen)
}

// stopTick invalidates the current tick chain. Any TickMsg still in flight
// from the old generation is dropped by the generation check in the TickMsg
// handler, so no stale chain can survive a stop.
func (m *Model) stopTick() {
	m.tickRunning = false
	m.tickGen++
}

// clearRunning marks the chain as ended WITHOUT bumping the generation.
// Use when a chain dies naturally (a TickMsg observed that playback is no
// longer active). Use stopTick instead when playback is stopped externally,
// so in-flight ticks from the old generation are also invalidated.
func (m *Model) clearRunning() {
	m.tickRunning = false
}
