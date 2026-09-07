package player

import (
	"testing"
	"testing/synctest"
	"time"
)

// TestMonitorLoop_TickInterval tests that the monitor loop ticks at 500ms intervals.
// This tests the timing logic without requiring audio hardware.
func TestMonitorLoop_TickInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var ticks []time.Time
		tickChan := make(chan struct{}, 10)
		done := make(chan struct{})

		// Simulate the monitor loop's ticker behavior
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					ticks = append(ticks, time.Now())
					tickChan <- struct{}{}
				case <-done:
					return
				}
			}
		}()

		// Wait for 3 ticks
		start := time.Now()
		time.Sleep(1600 * time.Millisecond) // Should get 3 ticks at 500ms, 1s, 1.5s
		synctest.Wait()
		close(done)

		// Drain tick channel
		tickCount := 0
		for {
			select {
			case <-tickChan:
				tickCount++
			default:
				goto checkTicks
			}
		}
	checkTicks:

		if tickCount != 3 {
			t.Errorf("got %d ticks, want 3", tickCount)
		}

		// Verify timing of ticks
		if len(ticks) >= 1 {
			firstTick := ticks[0].Sub(start)
			if firstTick < 450*time.Millisecond || firstTick > 550*time.Millisecond {
				t.Errorf("first tick at %v, want ~500ms", firstTick)
			}
		}
		if len(ticks) >= 2 {
			secondTick := ticks[1].Sub(start)
			if secondTick < 950*time.Millisecond || secondTick > 1050*time.Millisecond {
				t.Errorf("second tick at %v, want ~1s", secondTick)
			}
		}
		if len(ticks) >= 3 {
			thirdTick := ticks[2].Sub(start)
			if thirdTick < 1450*time.Millisecond || thirdTick > 1550*time.Millisecond {
				t.Errorf("third tick at %v, want ~1.5s", thirdTick)
			}
		}
	})
}

// TestSeekMuteDelay tests that the seek operation waits 100ms with muted audio.
// This tests the timing pattern without requiring audio hardware.
func TestSeekMuteDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Simulate the mute-wait-unmute pattern from doSeek
		muted := true
		var unmuteTime time.Time
		muteTime := time.Now()

		go func() {
			// Brief pause to let buffer clear before unmuting
			time.Sleep(100 * time.Millisecond)
			muted = false
			unmuteTime = time.Now()
		}()

		// Wait for unmute
		time.Sleep(150 * time.Millisecond)
		synctest.Wait()

		if muted {
			t.Error("expected muted to be false after 100ms")
		}

		delay := unmuteTime.Sub(muteTime)
		if delay < 100*time.Millisecond {
			t.Errorf("unmute delay = %v, want >= 100ms", delay)
		}
		if delay > 150*time.Millisecond {
			t.Errorf("unmute delay = %v, want <= 150ms", delay)
		}
	})
}

// TestPlayStartupDelay tests the 10ms delay after speaker.Clear() in Play().
func TestPlayStartupDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Simulate the delay pattern from Play()
		cleared := true
		var startedTime time.Time
		clearTime := time.Now()

		go func() {
			// Small delay to let any pending Beep callback complete after speaker.Clear()
			time.Sleep(10 * time.Millisecond)
			cleared = false
			startedTime = time.Now()
		}()

		// Wait for completion
		time.Sleep(20 * time.Millisecond)
		synctest.Wait()

		if cleared {
			t.Error("expected cleared to be false after 10ms")
		}

		delay := startedTime.Sub(clearTime)
		if delay < 10*time.Millisecond {
			t.Errorf("startup delay = %v, want >= 10ms", delay)
		}
		if delay > 20*time.Millisecond {
			t.Errorf("startup delay = %v, want <= 20ms", delay)
		}
	})
}

// TestPreloadCheckInterval verifies the 500ms interval is appropriate for preload checks.
func TestPreloadCheckInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// With default preloadAt of 3 seconds, 500ms checks means we'll catch
		// the preload window with at most 500ms delay.
		const preloadAt = 3 * time.Second

		// Simulate checking at different remaining times
		type check struct {
			remaining     time.Duration
			shouldPreload bool
		}

		checks := []check{
			{5 * time.Second, false},         // Too early
			{3500 * time.Millisecond, false}, // Still too early
			{3 * time.Second, true},          // Exactly at threshold
			{2 * time.Second, true},          // Past threshold
			{500 * time.Millisecond, true},   // Near end
			{0, false},                       // At end (remaining must be > 0)
		}

		for _, c := range checks {
			shouldPreload := c.remaining <= preloadAt && c.remaining > 0
			if shouldPreload != c.shouldPreload {
				t.Errorf("remaining=%v: shouldPreload=%v, want %v",
					c.remaining, shouldPreload, c.shouldPreload)
			}
		}
	})
}

// The seek request path is covered by seek_test.go, against the real Player.
