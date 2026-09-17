package client

import (
	"testing"
	"time"
)

// The provider resolves request_timeout and max_retries and passes them here,
// so the fields have to reach the HTTP client rather than being accepted and
// ignored. This lives in the internal test package because the client keeps
// its retryablehttp client unexported.
func TestConfigTuningReachesTheHTTPClient(t *testing.T) {
	// Any non-empty credential will do; New only needs one to be present.
	const adminKey = "unit-test-admin"

	timeout := 12 * time.Second
	retries := 0

	c, err := New(Config{AdminAPIKey: adminKey, RequestTimeout: &timeout, MaxRetries: &retries})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := c.http.HTTPClient.Timeout; got != timeout {
		t.Fatalf("timeout = %v, want %v", got, timeout)
	}
	if got := c.http.RetryMax; got != retries {
		t.Fatalf("RetryMax = %d, want %d (zero must mean one attempt, not the default)", got, retries)
	}

	// Unset must leave the documented defaults in place.
	d, err := New(Config{AdminAPIKey: adminKey})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := d.http.HTTPClient.Timeout; got != defaultTimeout {
		t.Fatalf("default timeout = %v, want %v", got, defaultTimeout)
	}
	if got := d.http.RetryMax; got != defaultRetries {
		t.Fatalf("default RetryMax = %d, want %d", got, defaultRetries)
	}
}
