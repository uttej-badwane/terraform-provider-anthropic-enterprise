package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Every attribute that decides where a secret or a signing key travels must
// refuse a cleartext URL. Each case is its own test function because a step
// that expects an error is replayed by the post-test destroy.

var httpsRequired = regexp.MustCompile(`must\s+start\s+with\s+https://`)

func TestAccHTTPSRequired_federationIssuerJWKSURL(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_federation_issuer" "test" {
  name       = "tf-acc-http-jwks"
  issuer_url = "https://issuer.example.com"
  jwks = {
    type = "explicit_url"
    url  = "http://issuer.example.com/.well-known/jwks.json"
  }
}
`,
			ExpectError: httpsRequired,
		}},
	})
}

func TestAccHTTPSRequired_federationIssuerDiscoveryBase(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_federation_issuer" "test" {
  name       = "tf-acc-http-discovery"
  issuer_url = "https://issuer.example.com"
  jwks = {
    type           = "discovery"
    discovery_base = "http://issuer.example.com"
  }
}
`,
			ExpectError: httpsRequired,
		}},
	})
}

func TestAccHTTPSRequired_vaultCredentialStaticBearer(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_vault_credential" "test" {
  vault_id = "vlt_01Example"
  static_bearer = {
    mcp_server_url = "http://mcp.example.com/sse"
    token          = "example-token"
  }
}
`,
			ExpectError: httpsRequired,
		}},
	})
}

func TestAccHTTPSRequired_vaultCredentialMCPOAuth(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_vault_credential" "test" {
  vault_id = "vlt_01Example"
  mcp_oauth = {
    mcp_server_url = "http://oauth-mcp.example.com/sse"
    access_token   = "example-token"
  }
}
`,
			ExpectError: httpsRequired,
		}},
	})
}

func TestAccHTTPSRequired_vaultCredentialTokenEndpoint(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_vault_credential" "test" {
  vault_id = "vlt_01Example"
  mcp_oauth = {
    mcp_server_url = "https://oauth-mcp.example.com/sse"
    access_token   = "example-token"
    refresh = {
      token_endpoint = "http://idp.example.com/oauth/token"
      client_id      = "example-client"
      refresh_token  = "example-refresh"
    }
  }
}
`,
			ExpectError: httpsRequired,
		}},
	})
}

func TestAccHTTPSRequired_deploymentRepositoryURL(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_deployment" "test" {
  name           = "tf-acc-http-repo"
  agent_id       = "agent_01Example"
  environment_id = "env_01Example"
  initial_events = jsonencode([{ type = "user.message", content = "hello" }])
  github_repositories = [{
    url                 = "http://github.example.com/example-org/example-repo"
    authorization_token = "example-github-token"
  }]
}
`,
			ExpectError: httpsRequired,
		}},
	})
}
