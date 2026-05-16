// internal/app/cpu_tick_repro_test.go
//
// Reproduction for issue #28 ("high cpu usage", builds up over time).
//
// Hypothesis: every track change while playing seeds a NEW, independent,
// self-sustaining 1-second TickCmd chain. Nothing ever cancels old chains, so
// the number of concurrent tickers grows by one on every track change. Each
// tick triggers a full Update+View, so per-second CPU work grows linearly with
// the number of tracks played in a session.
//
// This test drives the real Model.Update and counts how many independent
// TickMsg-producing chains are live. It uses testing/synctest so tea.Tick
// timers resolve deterministically in virtual time.
package app

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/llehouerou/waves/internal/playback"
	"github.com/llehouerou/waves/internal/player"
)

// countTickChains executes cmd (recursively flattening tea.Batch) under
// synctest virtual time and returns how many leaf commands resolve to a
// TickMsg. Each such leaf is one independent self-sustaining ticker chain.
//
// Leaves that stay durably blocked (e.g. WatchServiceEvents waiting on the
// subscription) produce no result and are not counted. Immediate non-tick
// leaves (e.g. the AlbumArtUpdateMsg closure) are ignored.
func countTickChains(t *testing.T, cmd tea.Cmd) int {
	t.Helper()
	if cmd == nil {
		return 0
	}
	var (
		mu    sync.Mutex
		count int
	)
	var walk func(c tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil {
			return
		}
		res := make(chan tea.Msg, 1)
		go func() {
			defer func() { _ = recover() }()
			res <- c()
		}()
		// Advance virtual time past TickCmd's 1s tea.Tick timer. A timer-blocked
		// goroutine is "durably blocked", so synctest.Wait() alone returns before
		// it fires; sleeping in virtual time lets the fake clock advance and the
		// leaf resolve. Non-tick leaves either resolve immediately or stay
		// durably blocked (WatchServiceEvents) and fall through to default.
		time.Sleep(2 * time.Second)
		synctest.Wait()
		select {
		case msg := <-res:
			switch m := msg.(type) {
			case tea.BatchMsg:
				for _, sub := range m {
					walk(sub)
				}
			case TickMsg:
				mu.Lock()
				count++
				mu.Unlock()
			}
		default:
			// Leaf still durably blocked (e.g. WatchServiceEvents): not a tick.
		}
	}
	walk(cmd)
	return count
}

func TestCPU_TickChainsAccumulatePerTrackChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := newIntegrationTestModel()
		defer func() {
			_ = m.PlaybackService.Close()
			synctest.Wait() // let blocked WatchServiceEvents goroutines exit
		}()

		m.PlaybackService.AddTracks(
			playback.Track{Path: "/a.mp3", Artist: "A", Title: "A", Album: "Alb"},
			playback.Track{Path: "/b.mp3", Artist: "B", Title: "B", Album: "Alb"},
		)
		mock, ok := m.PlaybackService.Player().(*player.Mock)
		if !ok {
			t.Fatal("expected mock player")
		}
		mock.SetState(player.Playing)

		// 1. Playback starts (stopped -> playing) seeds the first tick chain.
		mdl, cmd := updateModel(t, m, ServiceStateChangedMsg{
			Previous: int(playback.StateStopped),
			Current:  int(playback.StatePlaying),
		})
		m = mdl
		chains := countTickChains(t, cmd)

		// 2. Each track change while playing seeds ANOTHER chain (the bug).
		const trackChanges = 20
		for range trackChanges {
			mdl, cmd = updateModel(t, m, ServiceTrackChangedMsg{
				PreviousIndex: 0,
				CurrentIndex:  1,
			})
			m = mdl
			chains += countTickChains(t, cmd)
		}

		// 3. Each chain is self-sustaining: one TickMsg re-arms exactly one tick.
		_, tickCmd := updateModel(t, m, TickMsg(time.Now()))
		if got := countTickChains(t, tickCmd); got != 1 {
			t.Fatalf("a TickMsg while playing should re-arm exactly 1 tick, got %d", got)
		}

		// With `trackChanges` transitions the Bubble Tea runtime now delivers
		// `chains` TickMsgs every second, each causing a full Update+View.
		if chains != 1 {
			t.Fatalf(
				"tick chains = %d after %d track changes => %d Update+View per second (want 1, bounded).\n"+
					"CPU grows linearly with the number of tracks played in a session (issue #28).",
				chains, trackChanges, chains)
		}
	})
}
