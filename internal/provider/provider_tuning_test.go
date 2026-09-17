package provider

import (
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestTuningOrEnv(t *testing.T) {
	dur := func(s string) time.Duration {
		d, err := time.ParseDuration(s)
		if err != nil {
			t.Fatalf("bad test duration %q: %v", s, err)
		}
		return d
	}

	cases := []struct {
		name        string
		timeout     types.String
		retries     types.Int64
		env         map[string]string
		wantTimeout *time.Duration
		wantRetries *int
		wantErr     bool
	}{
		{
			name:    "unset leaves both at the client defaults",
			timeout: types.StringNull(),
			retries: types.Int64Null(),
		},
		{
			name:        "values from configuration",
			timeout:     types.StringValue("90s"),
			retries:     types.Int64Value(2),
			wantTimeout: ptrTo(dur("90s")),
			wantRetries: ptrTo(2),
		},
		{
			name:        "zero retries is honoured, not treated as unset",
			timeout:     types.StringNull(),
			retries:     types.Int64Value(0),
			wantRetries: ptrTo(0),
		},
		{
			name:        "values from the environment",
			timeout:     types.StringNull(),
			retries:     types.Int64Null(),
			env:         map[string]string{"ANTHROPIC_REQUEST_TIMEOUT": "2m", "ANTHROPIC_MAX_RETRIES": "1"},
			wantTimeout: ptrTo(dur("2m")),
			wantRetries: ptrTo(1),
		},
		{
			name:        "configuration wins over the environment",
			timeout:     types.StringValue("5s"),
			retries:     types.Int64Value(7),
			env:         map[string]string{"ANTHROPIC_REQUEST_TIMEOUT": "2m", "ANTHROPIC_MAX_RETRIES": "1"},
			wantTimeout: ptrTo(dur("5s")),
			wantRetries: ptrTo(7),
		},
		{
			name:    "a timeout that is not a duration is an error",
			timeout: types.StringValue("ninety"),
			retries: types.Int64Null(),
			wantErr: true,
		},
		{
			name:    "a zero timeout is an error",
			timeout: types.StringValue("0s"),
			retries: types.Int64Null(),
			wantErr: true,
		},
		{
			name:    "a negative timeout is an error",
			timeout: types.StringValue("-5s"),
			retries: types.Int64Null(),
			wantErr: true,
		},
		{
			name:    "a non-numeric retry count from the environment is an error",
			timeout: types.StringNull(),
			retries: types.Int64Null(),
			env:     map[string]string{"ANTHROPIC_MAX_RETRIES": "lots"},
			wantErr: true,
		},
		{
			name:    "a negative retry count from the environment is an error",
			timeout: types.StringNull(),
			retries: types.Int64Null(),
			env:     map[string]string{"ANTHROPIC_MAX_RETRIES": "-1"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			var diags diag.Diagnostics
			timeout, retries := tuningOrEnv(providerModel{RequestTimeout: tc.timeout, MaxRetries: tc.retries}, &diags)

			if tc.wantErr {
				if !diags.HasError() {
					t.Fatalf("expected an error, got timeout=%v retries=%v", timeout, retries)
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags.Errors())
			}
			assertDurationPtr(t, "timeout", timeout, tc.wantTimeout)
			assertIntPtr(t, "retries", retries, tc.wantRetries)
		})
	}
}

func ptrTo[T any](v T) *T { return &v }

func assertDurationPtr(t *testing.T, label string, got, want *time.Duration) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Fatalf("%s: got %v, want nil so the client default applies", label, *got)
	case want != nil && got == nil:
		t.Fatalf("%s: got nil, want %v", label, *want)
	case want != nil && *got != *want:
		t.Fatalf("%s: got %v, want %v", label, *got, *want)
	}
}

func assertIntPtr(t *testing.T, label string, got, want *int) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Fatalf("%s: got %d, want nil so the client default applies", label, *got)
	case want != nil && got == nil:
		t.Fatalf("%s: got nil, want %d", label, *want)
	case want != nil && *got != *want:
		t.Fatalf("%s: got %d, want %d", label, *got, *want)
	}
}

// Each error case is its own test function: the post-test destroy replays the
// last configuration, so an expected error cannot share a function with a
// successful step.

func TestAccProviderTuning_accepted(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
provider "anthropic" {
  request_timeout = "90s"
  max_retries     = 0
}
data "anthropic_organization" "test" {}
`,
		}},
	})
}

func TestAccProviderTuning_rejectsUnparseableTimeout(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
provider "anthropic" {
  request_timeout = "ninety"
}
data "anthropic_organization" "test" {}
`,
			ExpectError: regexp.MustCompile(`not\s+a\s+duration`),
		}},
	})
}

func TestAccProviderTuning_rejectsNegativeRetries(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
provider "anthropic" {
  max_retries = -1
}
data "anthropic_organization" "test" {}
`,
			ExpectError: regexp.MustCompile(`(?i)at\s+least|must\s+be`),
		}},
	})
}
