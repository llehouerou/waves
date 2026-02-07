package lastfm

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestWaitForAuthCallbackCmd_ReceivesToken(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tokenChan := make(chan string, 1)
		cmd := WaitForAuthCallbackCmd(tokenChan)

		// Send token before timeout
		tokenChan <- "test-token-123"

		msg := cmd()
		result, ok := msg.(TokenReceivedMsg)
		if !ok {
			t.Fatalf("expected TokenReceivedMsg, got %T", msg)
		}
		if result.Token != "test-token-123" {
			t.Errorf("Token = %q, want %q", result.Token, "test-token-123")
		}
	})
}

func TestWaitForAuthCallbackCmd_Timeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tokenChan := make(chan string)
		cmd := WaitForAuthCallbackCmd(tokenChan)

		// Run cmd in goroutine since it blocks
		type resultOrError struct {
			result TokenReceivedMsg
			ok     bool
		}
		done := make(chan resultOrError)
		go func() {
			msg := cmd()
			r, ok := msg.(TokenReceivedMsg)
			done <- resultOrError{r, ok}
		}()

		// Advance time past the 5 minute timeout
		time.Sleep(5*time.Minute + time.Second)
		synctest.Wait()

		res := <-done
		if !res.ok {
			t.Fatal("expected TokenReceivedMsg")
		}
		if res.result.Token != "" {
			t.Errorf("expected empty token on timeout, got %q", res.result.Token)
		}
	})
}

func TestWaitForAuthCallbackCmd_TokenBeforeTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tokenChan := make(chan string)
		cmd := WaitForAuthCallbackCmd(tokenChan)

		// Run cmd in goroutine
		type resultOrError struct {
			result TokenReceivedMsg
			ok     bool
		}
		done := make(chan resultOrError)
		go func() {
			msg := cmd()
			r, ok := msg.(TokenReceivedMsg)
			done <- resultOrError{r, ok}
		}()

		// Wait 2 minutes then send token (before 5 min timeout)
		time.Sleep(2 * time.Minute)
		tokenChan <- "delayed-token"

		synctest.Wait()
		res := <-done
		if !res.ok {
			t.Fatal("expected TokenReceivedMsg")
		}
		if res.result.Token != "delayed-token" {
			t.Errorf("Token = %q, want %q", res.result.Token, "delayed-token")
		}
	})
}
