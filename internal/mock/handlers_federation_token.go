package mock

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// MockFederationAssertion is the assertion the mock accepts. Any other value is
// denied, so tests can exercise both outcomes.
const MockFederationAssertion = "mock.assertion.accepted"

// MintedTokenPrefix matches the real service, which returns tokens prefixed
// sk-ant-oat01-.
//
//nolint:gosec // G101: the public prefix minted tokens carry, not a credential.
const MintedTokenPrefix = "sk-ant-oat01-"

// exchangeFederationToken models POST /v1/oauth/token.
//
// The endpoint takes no credential: the assertion is what authenticates the
// caller. The mock therefore rejects a request that carries one, which is what
// keeps the provider honest about never attaching a key here.
func (s *Server) exchangeFederationToken(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Api-Key") != "" || r.Header.Get("Authorization") != "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error",
			"the token exchange takes no credential; the assertion authenticates the caller")
		return
	}
	beta := r.Header.Get("Anthropic-Beta")
	if !strings.Contains(beta, "oidc-federation-2026-04-01") {
		writeError(w, http.StatusBadRequest, "invalid_request_error",
			"missing the oidc-federation beta header")
		return
	}

	var in client.FederationTokenExchange
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "malformed body")
		return
	}
	if in.GrantType != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "unsupported grant_type")
		return
	}
	for name, v := range map[string]string{
		"assertion":          in.Assertion,
		"federation_rule_id": in.FederationRuleID,
		"organization_id":    in.OrganizationID,
		"service_account_id": in.ServiceAccountID,
	} {
		if v == "" {
			writeError(w, http.StatusBadRequest, "invalid_request_error", name+" is required")
			return
		}
	}
	if in.WorkspaceID != "" && in.WorkspaceID != "default" && !strings.HasPrefix(in.WorkspaceID, "wrkspc_") {
		writeError(w, http.StatusBadRequest, "invalid_request_error",
			"workspace_id must be a wrkspc_ id or the literal default")
		return
	}

	// Every denial is the same opaque 401, as the real service does, so a
	// caller cannot probe rule configuration by comparing messages.
	if in.Assertion != MockFederationAssertion {
		writeError(w, http.StatusUnauthorized, "authentication_error", "Authentication failed")
		return
	}

	writeJSON(w, client.FederationToken{
		AccessToken: MintedTokenPrefix + "mockmintedtoken",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		Scope:       "workspace:inference",
	})
}
