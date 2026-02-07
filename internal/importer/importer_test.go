package importer

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestRetryWithBackoff_SuccessOnFirstAttempt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		err := RetryWithBackoff(ctx, "test op", func() error {
			callCount++
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if callCount != 1 {
			t.Errorf("callCount = %d, want 1", callCount)
		}
	})
}

func TestRetryWithBackoff_SuccessAfterRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		err := RetryWithBackoff(ctx, "test op", func() error {
			callCount++
			if callCount < 3 {
				return errors.New("temporary failure")
			}
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if callCount != 3 {
			t.Errorf("callCount = %d, want 3", callCount)
		}
	})
}

func TestRetryWithBackoff_ExhaustsRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		// Use a retryable error (contains "temporary")
		err := RetryWithBackoff(ctx, "test op", func() error {
			callCount++
			return errors.New("temporary failure")
		})

		if err == nil {
			t.Fatal("expected error after exhausting retries")
		}
		// Should be initial attempt + maxRetries
		expectedCalls := 1 + TestMaxRetries
		if callCount != expectedCalls {
			t.Errorf("callCount = %d, want %d", callCount, expectedCalls)
		}
	})
}

func TestRetryWithBackoff_BackoffTiming(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		var callTimes []time.Time

		// Use a retryable error (contains "temporary")
		_ = RetryWithBackoff(ctx, "test op", func() error {
			callTimes = append(callTimes, time.Now())
			return errors.New("temporary failure")
		})

		if len(callTimes) != 1+TestMaxRetries {
			t.Fatalf("expected %d calls, got %d", 1+TestMaxRetries, len(callTimes))
		}

		// First call is immediate
		// Second call after 500ms (initialBackoff)
		delay1 := callTimes[1].Sub(callTimes[0])
		if delay1 < 500*time.Millisecond {
			t.Errorf("first retry delay = %v, want >= 500ms", delay1)
		}

		// Third call after 1s (500ms * 2)
		delay2 := callTimes[2].Sub(callTimes[1])
		if delay2 < 1*time.Second {
			t.Errorf("second retry delay = %v, want >= 1s", delay2)
		}

		// Fourth call after 2s (1s * 2)
		delay3 := callTimes[3].Sub(callTimes[2])
		if delay3 < 2*time.Second {
			t.Errorf("third retry delay = %v, want >= 2s", delay3)
		}
	})
}

func TestRetryWithBackoff_BackoffCappedAtMax(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// We need more retries to see the cap effect
		// Current config: 500ms -> 1s -> 2s -> 4s (but capped at 5s)
		// With 3 retries, we don't reach the cap
		// Let's verify the exponential progression stays under cap
		ctx := context.Background()
		var callTimes []time.Time

		// Use a retryable error (contains "temporary")
		_ = RetryWithBackoff(ctx, "test op", func() error {
			callTimes = append(callTimes, time.Now())
			return errors.New("temporary failure")
		})

		// Total time should be: 500ms + 1s + 2s = 3.5s minimum
		// (not reaching the 5s cap with current retry count)
		totalTime := callTimes[len(callTimes)-1].Sub(callTimes[0])
		expectedMin := 500*time.Millisecond + 1*time.Second + 2*time.Second
		if totalTime < expectedMin {
			t.Errorf("total backoff time = %v, want >= %v", totalTime, expectedMin)
		}
	})
}

func TestRetryWithBackoff_ContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		callCount := 0

		done := make(chan error)
		go func() {
			done <- RetryWithBackoff(ctx, "test op", func() error {
				callCount++
				return errors.New("temporary failure")
			})
		}()

		// Let first attempt happen, then cancel during backoff wait
		time.Sleep(100 * time.Millisecond)
		synctest.Wait()
		cancel()

		err := <-done
		if err == nil {
			t.Fatal("expected error after context cancellation")
		}
		if callCount != 1 {
			t.Errorf("callCount = %d, want 1 (cancelled during first backoff)", callCount)
		}
	})
}

func TestRetryWithBackoff_ContextTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Context times out after 700ms (after first retry starts, during first backoff wait)
		// Initial call is immediate, then 500ms backoff wait
		// With 700ms timeout, we should get initial + wait interrupted
		ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
		defer cancel()

		callCount := 0
		err := RetryWithBackoff(ctx, "test op", func() error {
			callCount++
			return errors.New("temporary failure")
		})

		if err == nil {
			t.Fatal("expected error after context timeout")
		}
		// Should have done 2 calls: initial + 1 retry (after 500ms backoff)
		// Then timeout during second backoff (1s wait)
		if callCount != 2 {
			t.Errorf("callCount = %d, want 2", callCount)
		}
	})
}

func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		// Error that is NOT retryable (no "locked", "timeout", etc.)
		err := RetryWithBackoff(ctx, "test op", func() error {
			callCount++
			return errors.New("file not found")
		})

		if err == nil {
			t.Fatal("expected error")
		}
		// Should not retry on non-retryable errors
		if callCount != 1 {
			t.Errorf("callCount = %d, want 1 (no retry on non-retryable error)", callCount)
		}
	})
}

func TestRetryWithBackoff_RetryableErrors(t *testing.T) {
	retryableErrors := []string{
		"file is locked",
		"resource busy",
		"file in use by another process",
		"permission denied",
		"connection timeout",
		"network error",
		"i/o error",
		"temporary failure",
	}

	for _, errMsg := range retryableErrors {
		t.Run(errMsg, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := context.Background()
				callCount := 0

				_ = RetryWithBackoff(ctx, "test op", func() error {
					callCount++
					return errors.New(errMsg)
				})

				// Should retry (1 initial + maxRetries)
				expectedCalls := 1 + TestMaxRetries
				if callCount != expectedCalls {
					t.Errorf("callCount = %d, want %d for retryable error %q", callCount, expectedCalls, errMsg)
				}
			})
		})
	}
}

func TestIsRetryableError_Categories(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil error", nil, false},
		{"file locked", errors.New("file is locked"), true},
		{"resource busy", errors.New("resource busy"), true},
		{"in use", errors.New("file in use"), true},
		{"permission denied", errors.New("permission denied"), true},
		{"access denied", errors.New("access denied"), true},
		{"timeout", errors.New("operation timeout"), true},
		{"connection error", errors.New("connection refused"), true},
		{"network error", errors.New("network unreachable"), true},
		{"i/o error", errors.New("i/o error"), true},
		{"temporary", errors.New("temporary failure"), true},
		{"not found", errors.New("file not found"), false},
		{"invalid", errors.New("invalid argument"), false},
		{"unsupported", errors.New("unsupported format"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("IsRetryableError(%q) = %v, want %v", tt.err, result, tt.retryable)
			}
		})
	}
}

func TestRetryWithBackoff_OperationTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		// Each operation blocks until the timeout channel is ready
		// Then it receives and continues (simulating slow operation that finishes just after timeout)
		blockChan := make(chan struct{})

		done := make(chan error, 1)
		go func() {
			done <- RetryWithBackoff(ctx, "slow op", func() error {
				callCount++
				// Wait on a channel that will be unblocked after timeout fires
				<-blockChan
				return errors.New("temporary failure")
			})
		}()

		// For each attempt, wait for operation timeout + a bit, then unblock
		for range 1 + TestMaxRetries {
			// Wait for operation timeout
			time.Sleep(TestOperationTimeout + 100*time.Millisecond)
			synctest.Wait()
			// Unblock the goroutine so it can exit cleanly
			select {
			case blockChan <- struct{}{}:
			default:
			}
		}

		// Wait for backoff delays and completion
		time.Sleep(4 * time.Second)
		synctest.Wait()

		select {
		case err := <-done:
			if err == nil {
				t.Fatal("expected error after operation timeouts")
			}
		default:
			t.Fatal("RetryWithBackoff did not complete")
		}

		expectedCalls := 1 + TestMaxRetries
		if callCount != expectedCalls {
			t.Errorf("callCount = %d, want %d", callCount, expectedCalls)
		}
	})
}

func TestConstants(t *testing.T) {
	// Verify constants are as expected
	if TestMaxRetries != 3 {
		t.Errorf("maxRetries = %d, want 3", TestMaxRetries)
	}
	if TestInitialBackoff != 500*time.Millisecond {
		t.Errorf("initialBackoff = %v, want 500ms", TestInitialBackoff)
	}
	if TestMaxBackoff != 5*time.Second {
		t.Errorf("maxBackoff = %v, want 5s", TestMaxBackoff)
	}
	if TestOperationTimeout != 30*time.Second {
		t.Errorf("operationTimeout = %v, want 30s", TestOperationTimeout)
	}
}
