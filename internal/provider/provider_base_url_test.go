package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Each case is its own test function; see provider_credentials_test.go.

func TestAccProviderBaseURL_httpRemoteRejected(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
provider "anthropic" {
  base_url = "http://api.example.com"
}
data "anthropic_organization" "test" {}
`,
			ExpectError: regexp.MustCompile(`http\s+is\s+only\s+accepted\s+for\s+loopback`),
		}},
	})
}

func TestAccProviderBaseURL_userinfoRejected(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
provider "anthropic" {
  base_url = "https://svc-user@api.example.com"
}
data "anthropic_organization" "test" {}
`,
			ExpectError: regexp.MustCompile(`without\s+embedded\s+credentials`),
		}},
	})
}

func TestAccProviderBaseURL_relativeRejected(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
provider "anthropic" {
  base_url = "/v1"
}
data "anthropic_organization" "test" {}
`,
			ExpectError: regexp.MustCompile(`absolute\s+http\(s\)\s+URL`),
		}},
	})
}
