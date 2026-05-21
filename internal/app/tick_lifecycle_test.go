// internal/app/tick_lifecycle_test.go
//
// Regression for issue #28: exactly one tick chain survives playback
// transitions (stop->play, pause->resume). Reuses countTickChains from
// cpu_tick_repro_test.go.
package app

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/player"
)

func TestCPU_TickChainSurvivesTransitions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := newIntegrationTestModel()
		defer func() {
			_ = m.PlaybackService.Close()
			synctest.Wait()
		}()

		m.PlaybackService.AddTracks(
			playback.Track{Path: "/a.mp3", Artist: "A", Title: "A", Album: "Alb"},
		)
		mock, ok := m.PlaybackService.Player().(*player.Mock)
		if !ok {
			t.Fatal("expected mock player")
		}

		// Play -> exactly one chain.
		mock.SetState(player.Playing)
		mdl, cmd := updateModel(t, m, ServiceStateChangedMsg{
			Previous: int(playback.StateStopped),
			Current:  int(playback.StatePlaying),
		})
		m = mdl
		if got := countTickChains(t, cmd); got != 1 {
			t.Fatalf("after play: chains = %d, want 1", got)
		}

		// Stop -> the chain's next TickMsg (old gen) must not re-arm.
		staleGen := m.tickGen
		mock.SetState(player.Stopped)
		mdl, _ = updateModel(t, m, ServiceStateChangedMsg{
			Previous: int(playback.StatePlaying),
			Current:  int(playback.StateStopped),
		})
		m = mdl
		_, staleCmd := updateModel(t, m, TickMsg{Gen: staleGen, Time: time.Now()})
		if got := countTickChains(t, staleCmd); got != 0 {
			t.Fatalf("stale tick after stop re-armed %d chains, want 0", got)
		}

		// Play again -> exactly one fresh chain (not zero, not two).
		mock.SetState(player.Playing)
		mdl, cmd = updateModel(t, m, ServiceStateChangedMsg{
			Previous: int(playback.StateStopped),
			Current:  int(playback.StatePlaying),
		})
		m = mdl
		if got := countTickChains(t, cmd); got != 1 {
			t.Fatalf("after stop->play: chains = %d, want 1", got)
		}

		// Pause: the current chain's tick sees !IsPlaying and ends itself.
		mock.SetState(player.Paused)
		_, pausedCmd := updateModel(t, m, TickMsg{Gen: m.tickGen, Time: time.Now()})
		if got := countTickChains(t, pausedCmd); got != 0 {
			t.Fatalf("tick while paused re-armed %d chains, want 0", got)
		}
		// Two observations of the same paused transition: the first call
		// above confirmed no re-arm command; this one confirms the returned
		// model has tickRunning cleared.
		mdl, _ = updateModel(t, m, TickMsg{Gen: m.tickGen, Time: time.Now()})
		m = mdl
		if m.tickRunning {
			t.Fatal("tickRunning should be false after a tick while paused")
		}

		// Resume -> exactly one chain again. The real service emits
		// Previous: StatePaused on resume; handleServiceStateChanged only
		// special-cases StateStopped, so paused/playing take the same path.
		mock.SetState(player.Playing)
		mdl, cmd = updateModel(t, m, ServiceStateChangedMsg{
			Previous: int(playback.StatePaused),
			Current:  int(playback.StatePlaying),
		})
		m = mdl
		if got := countTickChains(t, cmd); got != 1 {
			t.Fatalf("after pause->resume: chains = %d, want 1", got)
		}
	})
}

// Pausing must stop the tick chain immediately (no wasted Update+View while
// paused). The state-change handler bumps the generation and clears
// tickRunning, so any in-flight TickMsg from the old generation is dropped.
func TestCPU_PauseStopsTickImmediately(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := newIntegrationTestModel()
		defer func() {
			_ = m.PlaybackService.Close()
			synctest.Wait()
		}()

		m.PlaybackService.AddTracks(
			playback.Track{Path: "/a.mp3", Artist: "A", Title: "A", Album: "Alb"},
		)
		mock, ok := m.PlaybackService.Player().(*player.Mock)
		if !ok {
			t.Fatal("expected mock player")
		}

		// Start playing -> one chain alive.
		mock.SetState(player.Playing)
		mdl, _ := updateModel(t, m, ServiceStateChangedMsg{
			Previous: int(playback.StateStopped),
			Current:  int(playback.StatePlaying),
		})
		m = mdl
		if !m.tickRunning {
			t.Fatal("expected tickRunning=true after play")
		}
		staleGen := m.tickGen

		// Playing -> Paused state change. The chain must end immediately, not
		// on its next tick: tickRunning=false and the generation bumps so any
		// in-flight TickMsg from the old generation is dropped.
		mock.SetState(player.Paused)
		mdl, _ = updateModel(t, m, ServiceStateChangedMsg{
			Previous: int(playback.StatePlaying),
			Current:  int(playback.StatePaused),
		})
		m = mdl
		if m.tickRunning {
			t.Fatal("pause should stop the tick chain immediately (no wasted ticks)")
		}
		if m.tickGen == staleGen {
			t.Fatal("pause should bump tickGen so in-flight ticks are dropped")
		}

		// A still-in-flight tick from the old generation must not re-arm.
		_, staleCmd := updateModel(t, m, TickMsg{Gen: staleGen, Time: time.Now()})
		if got := countTickChains(t, staleCmd); got != 0 {
			t.Fatalf("stale tick after pause re-armed %d chains, want 0", got)
		}
	})
}
