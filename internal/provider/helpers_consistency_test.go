package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// These helpers exist because the Admin API is eventually consistent: a member
// added a moment ago can be reported as absent by the very next update. The
// recognition of that state is a string match on the API's wording, which will
// break silently if it ever changes, so it is worth testing directly.
//
// Every case uses a window measured in milliseconds. The production window is
// twenty seconds and proving a timeout works must not take twenty seconds.

const (
	testTimeout  = 60 * time.Millisecond
	testInterval = 2 * time.Millisecond
)

func notInWorkspaceErr() error {
	return &client.APIError{
		StatusCode: 400,
		Type:       "invalid_request_error",
		Message:    "The specified user is not in the Workspace",
	}
}

func TestIsNotYetVisible(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"the propagation error", notInWorkspaceErr(), true},
		{
			"wrapped, because callers wrap",
			fmt.Errorf("adding member: %w", notInWorkspaceErr()),
			true,
		},
		{
			"matched case-insensitively",
			&client.APIError{StatusCode: 400, Message: "The specified user is NOT IN THE WORKSPACE"},
			true,
		},
		{
			"a different 400 is not propagation",
			&client.APIError{StatusCode: 400, Message: "workspace_role is invalid"},
			false,
		},
		{
			"right message, wrong status",
			&client.APIError{StatusCode: 500, Message: "The specified user is not in the Workspace"},
			false,
		},
		{"a 404 is a real absence", &client.APIError{StatusCode: 404, Message: "not found"}, false},
		{"not an API error at all", errors.New("The specified user is not in the Workspace"), false},
		{"no error", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isNotYetVisible(tc.err); got != tc.want {
				t.Fatalf("isNotYetVisible(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestRetryUntilVisibleReturnsOnFirstSuccess(t *testing.T) {
	t.Parallel()

	calls := 0
	got, err := retryUntilVisibleFor(context.Background(), testTimeout, testInterval,
		func() (string, error) {
			calls++
			return "ok", nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" || calls != 1 {
		t.Fatalf("got %q after %d calls, want %q after 1", got, calls, "ok")
	}
}

func TestRetryUntilVisibleRetriesThenSucceeds(t *testing.T) {
	t.Parallel()

	calls := 0
	got, err := retryUntilVisibleFor(context.Background(), testTimeout, testInterval,
		func() (int, error) {
			calls++
			if calls < 3 {
				return 0, notInWorkspaceErr()
			}
			return calls, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 3 || calls != 3 {
		t.Fatalf("succeeded on call %d (returned %d), want 3", calls, got)
	}
}

// An error that is not the propagation error must surface immediately. Retrying
// a genuine validation failure would only delay the report.
func TestRetryUntilVisibleDoesNotRetryOtherErrors(t *testing.T) {
	t.Parallel()

	other := &client.APIError{StatusCode: 400, Message: "workspace_role is invalid"}
	calls := 0
	_, err := retryUntilVisibleFor(context.Background(), testTimeout, testInterval,
		func() (struct{}, error) {
			calls++
			return struct{}{}, other
		})
	if !errors.Is(err, error(other)) {
		t.Fatalf("got %v, want the original error back", err)
	}
	if calls != 1 {
		t.Fatalf("called %d times, want 1", calls)
	}
}

// At the deadline the caller gets the API's own error, not a synthesised one,
// so the diagnostic says what the API actually said.
func TestRetryUntilVisibleGivesUpWithTheAPIError(t *testing.T) {
	t.Parallel()

	start := time.Now()
	calls := 0
	_, err := retryUntilVisibleFor(context.Background(), testTimeout, testInterval,
		func() (struct{}, error) {
			calls++
			return struct{}{}, notInWorkspaceErr()
		})
	elapsed := time.Since(start)

	if !isNotYetVisible(err) {
		t.Fatalf("got %v, want the propagation error", err)
	}
	if calls < 2 {
		t.Fatalf("called %d times, want more than one attempt before giving up", calls)
	}
	if elapsed > time.Second {
		t.Fatalf("took %v to honour a %v window", elapsed, testTimeout)
	}
}

func TestRetryUntilVisibleHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	start := time.Now()
	_, err := retryUntilVisibleFor(ctx, time.Minute, testInterval, func() (struct{}, error) {
		calls++
		if calls == 2 {
			cancel()
		}
		return struct{}{}, notInWorkspaceErr()
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	// A one-minute window must not be waited out once the context is done.
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("took %v to notice cancellation", elapsed)
	}
}

func TestWaitUntilReturnsWhenCheckPasses(t *testing.T) {
	t.Parallel()

	calls := 0
	start := time.Now()
	waitUntilFor(context.Background(), time.Minute, testInterval, func() bool {
		calls++
		return calls >= 3
	})

	if calls != 3 {
		t.Fatalf("checked %d times, want 3", calls)
	}
	// It must return on convergence rather than waiting out the window.
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("took %v after the check passed", elapsed)
	}
}

// waitUntil never reports failure: a window that elapses without convergence
// leaves the difference for the next plan rather than failing the apply.
func TestWaitUntilGivesUpQuietly(t *testing.T) {
	t.Parallel()

	calls := 0
	start := time.Now()
	waitUntilFor(context.Background(), testTimeout, testInterval, func() bool {
		calls++
		return false
	})
	elapsed := time.Since(start)

	if calls < 2 {
		t.Fatalf("checked %d times, want repeated polling", calls)
	}
	if elapsed > time.Second {
		t.Fatalf("took %v to honour a %v window", elapsed, testTimeout)
	}
}

func TestWaitUntilHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	start := time.Now()
	waitUntilFor(ctx, time.Minute, testInterval, func() bool {
		calls++
		if calls == 2 {
			cancel()
		}
		return false
	})

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("took %v to notice cancellation", elapsed)
	}
}

// The wrappers must carry the package defaults through, or production code
// would silently run with a zero window.
func TestDefaultWindowIsWiredThrough(t *testing.T) {
	t.Parallel()

	if consistencyTimeout <= 0 || consistencyInterval <= 0 {
		t.Fatalf("defaults are not positive: timeout %v, interval %v", consistencyTimeout, consistencyInterval)
	}
	if consistencyInterval >= consistencyTimeout {
		t.Fatalf("interval %v is not shorter than timeout %v, so only one attempt would be made",
			consistencyInterval, consistencyTimeout)
	}

	calls := 0
	if _, err := retryUntilVisible(context.Background(), func() (struct{}, error) {
		calls++
		return struct{}{}, nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("retryUntilVisible called op %d times, want 1", calls)
	}

	checked := false
	waitUntil(context.Background(), func() bool {
		checked = true
		return true
	})
	if !checked {
		t.Fatal("waitUntil never ran the check")
	}
}
