package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// The speed dimension is gated on a beta header. The header must go out when
// the caller asks for the dimension and must not go out otherwise: a beta can
// change without a deprecation period, so opting every reader of this report
// into one they did not ask for is a behaviour change nobody requested.
//
// Asserting the absence is the half that would actually catch a regression.
// Sending the header unconditionally passes every test that only checks it is
// present.
func TestUsageReportSendsFastModeBetaOnlyWhenAsked(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		params   client.UsageReportParams
		wantBeta bool
	}{
		"no speed anything": {
			params:   client.UsageReportParams{StartingAt: "2026-01-01T00:00:00Z"},
			wantBeta: false,
		},
		"grouped by another dimension": {
			params:   client.UsageReportParams{StartingAt: "2026-01-01T00:00:00Z", GroupBy: []string{"model", "workspace_id"}},
			wantBeta: false,
		},
		"filtered by speeds": {
			params:   client.UsageReportParams{StartingAt: "2026-01-01T00:00:00Z", Speeds: []string{"fast"}},
			wantBeta: true,
		},
		"grouped by speed": {
			params:   client.UsageReportParams{StartingAt: "2026-01-01T00:00:00Z", GroupBy: []string{"speed"}},
			wantBeta: true,
		},
		// Grouping by speed without filtering still needs the header, which is
		// the case a naive "only when Speeds is set" check would miss.
		"grouped by speed alongside others": {
			params:   client.UsageReportParams{StartingAt: "2026-01-01T00:00:00Z", GroupBy: []string{"model", "speed"}},
			wantBeta: true,
		},
	}

	// Accepted only by the httptest server below; never sent anywhere real.
	const testAdminKey = "sk-ant-admin-test" //nolint:gosec // fixture

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var gotBeta string
			var gotSpeeds []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotBeta = r.Header.Get("Anthropic-Beta")
				gotSpeeds = r.URL.Query()["speeds[]"]
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":[],"next_page":null}`))
			}))
			defer srv.Close()

			c, err := client.New(client.Config{BaseURL: srv.URL, AdminAPIKey: testAdminKey})
			if err != nil {
				t.Fatalf("client: %v", err)
			}
			if _, err := c.GetUsageReport(context.Background(), tc.params); err != nil {
				t.Fatalf("GetUsageReport: %v", err)
			}

			hasBeta := strings.Contains(gotBeta, client.BetaFastMode)
			if hasBeta != tc.wantBeta {
				t.Errorf("beta header %q: got fast-mode=%v, want %v", gotBeta, hasBeta, tc.wantBeta)
			}
			if len(tc.params.Speeds) != len(gotSpeeds) {
				t.Errorf("speeds[] = %v, want %v", gotSpeeds, tc.params.Speeds)
			}
		})
	}
}
