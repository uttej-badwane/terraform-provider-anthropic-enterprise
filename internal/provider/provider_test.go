package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/mock"
)

// testAccProtoV6ProviderFactories is used by every acceptance test.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"anthropic": providerserver.NewProtocol6WithError(New("test")()),
}

// testMock is the in-process Admin API used unless ANTHROPIC_ACC_LIVE=1.
var testMock *mock.Server

func TestMain(m *testing.M) {
	if os.Getenv("ANTHROPIC_ACC_LIVE") == "" {
		testMock = mock.NewServer()
		os.Setenv("ANTHROPIC_BASE_URL", testMock.URL)
		os.Setenv("ANTHROPIC_ADMIN_API_KEY", mock.AdminKey)
		os.Setenv("ANTHROPIC_AUTH_TOKEN", mock.OAuthToken)
		os.Setenv("ANTHROPIC_ENTERPRISE_API_KEY", mock.EnterpriseKey)
		os.Setenv("ANTHROPIC_COMPLIANCE_API_KEY", mock.ComplianceKey)
		os.Setenv("ANTHROPIC_ANALYTICS_API_KEY", mock.AnalyticsKey)
		os.Setenv("ANTHROPIC_API_KEY", mock.AgentsKey)
		os.Setenv("ANTHROPIC_WORKSPACE_ID", "wrkspc_000000000000")
	}
	code := m.Run()
	if testMock != nil {
		testMock.Close()
	}
	os.Exit(code)
}

// testAccPreCheck verifies the environment for acceptance tests.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("ANTHROPIC_ADMIN_API_KEY") == "" && os.Getenv("ANTHROPIC_AUTH_TOKEN") == "" && os.Getenv("ANTHROPIC_ENTERPRISE_API_KEY") == "" {
		t.Fatal("ANTHROPIC_ADMIN_API_KEY, ANTHROPIC_AUTH_TOKEN or ANTHROPIC_ENTERPRISE_API_KEY must be set for acceptance tests")
	}
}

// skipUnlessMock skips tests that must never run against a live organization.
func skipUnlessMock(t *testing.T) {
	t.Helper()
	if testMock == nil {
		t.Skip("mock-only test: skipped against a live organization")
	}
}

// skipUnlessLive skips tests that only make sense against a live organization.
func skipUnlessLive(t *testing.T) {
	t.Helper()
	if os.Getenv("ANTHROPIC_ACC_LIVE") == "" {
		t.Skip("live-only test: set ANTHROPIC_ACC_LIVE=1")
	}
}

const providerConfig = `
provider "anthropic" {}
`
