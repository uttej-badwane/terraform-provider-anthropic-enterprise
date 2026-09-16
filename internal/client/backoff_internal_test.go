package client

import (
	"net/http"
	"testing"
	"time"
)

// Retry-After is honoured up to RetryWaitMax and no further.
func TestClampedBackoffHonoursRetryAfterWithinBound(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"3"}}}
	if got := clampedBackoff(500*time.Millisecond, 20*time.Second, 0, resp); got != 3*time.Second {
		t.Fatalf("expected 3s, got %s", got)
	}
}

func TestClampedBackoffCapsRetryAfter(t *testing.T) {
	for _, h := range []string{"3600", "999999999", time.Now().Add(48 * time.Hour).UTC().Format(http.TimeFormat)} {
		resp := &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{"Retry-After": {h}}}
		if got := clampedBackoff(500*time.Millisecond, 20*time.Second, 0, resp); got != 20*time.Second {
			t.Fatalf("Retry-After %q: expected the 20s cap, got %s", h, got)
		}
	}
}

func TestClampedBackoffExponentialWithoutHeader(t *testing.T) {
	if got := clampedBackoff(500*time.Millisecond, 20*time.Second, 2, &http.Response{StatusCode: 500}); got != 2*time.Second {
		t.Fatalf("expected 2s for attempt 2, got %s", got)
	}
	if got := clampedBackoff(500*time.Millisecond, 20*time.Second, 10, nil); got != 20*time.Second {
		t.Fatalf("expected the cap for a late attempt, got %s", got)
	}
}
