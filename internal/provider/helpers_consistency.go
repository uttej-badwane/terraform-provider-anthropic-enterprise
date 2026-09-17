package provider

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// Workspace membership on the Admin API is eventually consistent: a member
// added a moment ago may not be visible to an update ("The specified user is
// not in the Workspace"). In addition the per-member GET endpoint is served
// from a cache that writes do not invalidate for tens of seconds, which is why
// the membership resources read through the list endpoint. These helpers make
// the resources wait for the store to converge.

const (
	consistencyTimeout  = 20 * time.Second
	consistencyInterval = 750 * time.Millisecond
)

// isNotYetVisible reports the 400 the API returns while a fresh membership
// has not propagated yet.
func isNotYetVisible(err error) bool {
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		return false
	}
	return strings.Contains(strings.ToLower(apiErr.Message), "not in the workspace")
}

// retryUntilVisible runs op, retrying while the API says the membership is
// not visible yet, until the consistency timeout elapses.
func retryUntilVisible[T any](ctx context.Context, op func() (T, error)) (T, error) {
	return retryUntilVisibleFor(ctx, consistencyTimeout, consistencyInterval, op)
}

// retryUntilVisibleFor is retryUntilVisible over an explicit window. op always
// runs at least once, and the error from the final attempt is the one returned,
// so a caller sees the API's own message rather than a synthesised timeout.
func retryUntilVisibleFor[T any](ctx context.Context, timeout, interval time.Duration, op func() (T, error)) (T, error) {
	deadline := time.Now().Add(timeout)
	for {
		out, err := op()
		if err == nil || !isNotYetVisible(err) || time.Now().After(deadline) {
			return out, err
		}
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// waitUntil polls check until it reports true or the consistency timeout
// elapses. It never fails: convergence problems surface as a normal diff on
// the next plan instead of a hard error.
func waitUntil(ctx context.Context, check func() bool) {
	waitUntilFor(ctx, consistencyTimeout, consistencyInterval, check)
}

// waitUntilFor is waitUntil over an explicit window. Like waitUntil it never
// reports failure: a window that elapses without convergence leaves the work
// for the next plan.
func waitUntilFor(ctx context.Context, timeout, interval time.Duration, check func() bool) {
	deadline := time.Now().Add(timeout)
	for !check() && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
