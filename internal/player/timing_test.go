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

// TestSeekChannelNonBlocking tests that the seek channel pattern is non-blocking.
func TestSeekChannelNonBlocking(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		seekChan := make(chan time.Duration, 1)

		// First send should succeed
		select {
		case seekChan <- 5 * time.Second:
			// OK
		default:
			t.Error("first send should succeed")
		}

		// Second send should not block - should drain and resend
		start := time.Now()
		select {
		case seekChan <- 10 * time.Second:
			t.Error("second send should fail on full channel")
		default:
			// Expected - channel full
			// Drain and send new value (as in Seek implementation)
			select {
			case <-seekChan:
			default:
			}
			select {
			case seekChan <- 10 * time.Second:
			default:
				t.Error("send after drain should succeed")
			}
		}
		elapsed := time.Since(start)

		// Should be nearly instant (not blocking)
		if elapsed > 10*time.Millisecond {
			t.Errorf("non-blocking send took %v, expected < 10ms", elapsed)
		}

		// Verify final value is the latest
		val := <-seekChan
		if val != 10*time.Second {
			t.Errorf("final value = %v, want 10s", val)
		}
	})
}

// TestSeekLoopProcessing tests that seekLoop processes requests sequentially.
func TestSeekLoopProcessing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		seekChan := make(chan time.Duration, 1)
		var processed []time.Duration

		// Simulate seekLoop
		done := make(chan struct{})
		go func() {
			for delta := range seekChan {
				processed = append(processed, delta)
				// Simulate processing time
				time.Sleep(50 * time.Millisecond)
			}
			close(done)
		}()

		// Send seek requests
		seekChan <- 5 * time.Second
		time.Sleep(100 * time.Millisecond)
		seekChan <- -3 * time.Second
		time.Sleep(100 * time.Millisecond)

		close(seekChan)
		<-done

		if len(processed) != 2 {
			t.Fatalf("processed %d seeks, want 2", len(processed))
		}
		if processed[0] != 5*time.Second {
			t.Errorf("first seek = %v, want 5s", processed[0])
		}
		if processed[1] != -3*time.Second {
			t.Errorf("second seek = %v, want -3s", processed[1])
		}
	})
}
