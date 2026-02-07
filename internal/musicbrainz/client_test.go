//nolint:bodyclose // Test file uses http.NoBody which doesn't require closing
package musicbrainz

import (
	"errors"
	"net/http"
	"testing"
	"testing/synctest"
	"time"
)

func TestClient_WaitForRateLimit_FirstRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Client{}

		start := time.Now()
		c.waitForRateLimit()
		elapsed := time.Since(start)

		// First request should not wait
		if elapsed > 10*time.Millisecond {
			t.Errorf("first request waited %v, expected no wait", elapsed)
		}
	})
}

func TestClient_WaitForRateLimit_EnforcesRateLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Client{}

		// First request
		c.waitForRateLimit()

		// Immediate second request should wait ~1 second
		start := time.Now()
		c.waitForRateLimit()
		elapsed := time.Since(start)

		if elapsed < 900*time.Millisecond {
			t.Errorf("second request only waited %v, expected ~1s", elapsed)
		}
	})
}

func TestClient_WaitForRateLimit_NoWaitAfterDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Client{}

		// First request
		c.waitForRateLimit()

		// Wait more than rate limit
		time.Sleep(rateLimitDur + 100*time.Millisecond)

		// Second request should not wait
		start := time.Now()
		c.waitForRateLimit()
		elapsed := time.Since(start)

		if elapsed > 10*time.Millisecond {
			t.Errorf("request after delay waited %v, expected no wait", elapsed)
		}
	})
}

func TestClient_WaitForRateLimit_MultipleRequests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Client{}

		start := time.Now()

		// Make 5 requests
		for range 5 {
			c.waitForRateLimit()
		}

		elapsed := time.Since(start)

		// Should take at least 4 seconds (first is instant, then 4 waits of 1s each)
		if elapsed < 4*time.Second {
			t.Errorf("5 requests took %v, expected at least 4s", elapsed)
		}
	})
}

// mockTransport is a mock http.RoundTripper for testing.
type mockTransport struct {
	responses []*http.Response
	errors    []error
	callCount int
}

func (m *mockTransport) RoundTrip(*http.Request) (*http.Response, error) {
	idx := m.callCount
	m.callCount++

	if idx < len(m.errors) && m.errors[idx] != nil {
		return nil, m.errors[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return nil, errors.New("no more responses configured")
}

func newMockResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       http.NoBody,
	}
}

func TestClient_DoRequestWithRetry_Success(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mock := &mockTransport{
			responses: []*http.Response{newMockResponse(http.StatusOK)},
		}
		c := &Client{
			httpClient: &http.Client{Transport: mock},
		}

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		resp, err := c.doRequestWithRetry(req)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if mock.callCount != 1 {
			t.Errorf("callCount = %d, want 1", mock.callCount)
		}
	})
}

func TestClient_DoRequestWithRetry_RetriesOn500(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mock := &mockTransport{
			responses: []*http.Response{
				newMockResponse(http.StatusInternalServerError),
				newMockResponse(http.StatusInternalServerError),
				newMockResponse(http.StatusOK), // Success on 3rd attempt
			},
		}
		c := &Client{
			httpClient: &http.Client{Transport: mock},
		}

		start := time.Now()
		req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		resp, err := c.doRequestWithRetry(req)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if mock.callCount != 3 {
			t.Errorf("callCount = %d, want 3", mock.callCount)
		}

		// Should have waited: 2s (first retry) + 4s (second retry) = 6s minimum
		// Plus rate limit waits after each retry
		if elapsed < 6*time.Second {
			t.Errorf("elapsed = %v, expected at least 6s for backoff", elapsed)
		}
	})
}

func TestClient_DoRequestWithRetry_ExhaustsRetries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mock := &mockTransport{
			responses: []*http.Response{
				newMockResponse(http.StatusInternalServerError),
				newMockResponse(http.StatusInternalServerError),
				newMockResponse(http.StatusInternalServerError),
				newMockResponse(http.StatusInternalServerError), // All 4 attempts fail
			},
		}
		c := &Client{
			httpClient: &http.Client{Transport: mock},
		}

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		resp, err := c.doRequestWithRetry(req)

		if err == nil {
			t.Fatal("expected error after exhausting retries")
		}
		if resp != nil {
			t.Error("expected nil response after exhausting retries")
		}
		if mock.callCount != 4 {
			t.Errorf("callCount = %d, want 4 (initial + 3 retries)", mock.callCount)
		}
	})
}

func TestClient_DoRequestWithRetry_NoRetryOn4xx(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mock := &mockTransport{
			responses: []*http.Response{newMockResponse(http.StatusNotFound)},
		}
		c := &Client{
			httpClient: &http.Client{Transport: mock},
		}

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		resp, err := c.doRequestWithRetry(req)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
		if mock.callCount != 1 {
			t.Errorf("callCount = %d, want 1 (no retry on 4xx)", mock.callCount)
		}
	})
}

func TestClient_DoRequestWithRetry_RetriesOnNetworkError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mock := &mockTransport{
			errors: []error{
				errors.New("connection refused"),
				errors.New("timeout"),
				nil, // Success on 3rd
			},
			responses: []*http.Response{
				nil,
				nil,
				newMockResponse(http.StatusOK),
			},
		}
		c := &Client{
			httpClient: &http.Client{Transport: mock},
		}

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		resp, err := c.doRequestWithRetry(req)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if mock.callCount != 3 {
			t.Errorf("callCount = %d, want 3", mock.callCount)
		}
	})
}

func TestClient_DoRequestWithRetry_BackoffTiming(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var callTimes []time.Time
		mock := &mockTransport{
			responses: []*http.Response{
				newMockResponse(http.StatusInternalServerError),
				newMockResponse(http.StatusInternalServerError),
				newMockResponse(http.StatusInternalServerError),
				newMockResponse(http.StatusInternalServerError),
			},
		}

		// Wrap to record call times
		originalTransport := mock
		c := &Client{
			httpClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					callTimes = append(callTimes, time.Now())
					return originalTransport.RoundTrip(req)
				}),
			},
		}

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
		_, _ = c.doRequestWithRetry(req)

		if len(callTimes) != 4 {
			t.Fatalf("expected 4 calls, got %d", len(callTimes))
		}

		// Check delays between calls (should be ~2s, ~4s, ~8s + rate limit)
		// First retry: 2s delay
		delay1 := callTimes[1].Sub(callTimes[0])
		if delay1 < 2*time.Second {
			t.Errorf("first retry delay = %v, want >= 2s", delay1)
		}

		// Second retry: 4s delay
		delay2 := callTimes[2].Sub(callTimes[1])
		if delay2 < 4*time.Second {
			t.Errorf("second retry delay = %v, want >= 4s", delay2)
		}

		// Third retry: 8s delay
		delay3 := callTimes[3].Sub(callTimes[2])
		if delay3 < 8*time.Second {
			t.Errorf("third retry delay = %v, want >= 8s", delay3)
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
