package client

import (
	"context"
	"fmt"
)

// The jwt-bearer grant type from RFC 7523, which the token exchange requires
// verbatim.
//
//nolint:gosec // G101: an RFC 7523 grant type identifier, not a credential.
const jwtBearerGrantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"

// maxAssertionBytes is the documented ceiling on the assertion JWT. Rejecting
// an oversized assertion here keeps a large body from being sent at all.
const maxAssertionBytes = 16 << 10

// FederationTokenExchange is a request to trade an OIDC assertion for a
// short-lived Anthropic token under a federation rule.
type FederationTokenExchange struct {
	GrantType string `json:"grant_type"`
	// Assertion is the OIDC JWT issued by the workload's identity provider.
	Assertion string `json:"assertion"`
	// FederationRuleID is the rule to evaluate the assertion against.
	FederationRuleID string `json:"federation_rule_id"`
	// OrganizationID is the organization UUID the rule belongs to.
	OrganizationID string `json:"organization_id"`
	// ServiceAccountID is the account the minted token acts as.
	ServiceAccountID string `json:"service_account_id"`
	// WorkspaceID scopes the minted token. Required when the rule is enabled
	// for more than one workspace; the literal "default" selects the
	// organization's default workspace.
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// FederationToken is a minted access token. The token itself is short-lived by
// design and is never written to Terraform state.
type FederationToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
}

// ExchangeFederationToken trades a signed OIDC assertion for a short-lived
// Anthropic access token.
//
// The call is deliberately unauthenticated: the assertion in the body is what
// proves the caller's identity, and the endpoint is the one place a credential
// is minted rather than presented. A denied exchange answers with an opaque
// 401 whatever the underlying reason, so the error is passed through unchanged
// rather than being interpreted.
func (c *Client) ExchangeFederationToken(ctx context.Context, in FederationTokenExchange) (*FederationToken, error) {
	if in.Assertion == "" {
		return nil, fmt.Errorf("assertion is required")
	}
	if len(in.Assertion) > maxAssertionBytes {
		return nil, fmt.Errorf("assertion is %d bytes, which exceeds the %d byte maximum", len(in.Assertion), maxAssertionBytes)
	}
	if in.FederationRuleID == "" || in.OrganizationID == "" || in.ServiceAccountID == "" {
		return nil, fmt.Errorf("federation_rule_id, organization_id and service_account_id are required")
	}
	in.GrantType = jwtBearerGrantType

	var out FederationToken
	if err := c.post(ctx, CredNone, "/v1/oauth/token", in, &out,
		withBeta("oauth-2025-04-20,oidc-federation-2026-04-01")); err != nil {
		return nil, err
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("token exchange returned no access_token")
	}
	return &out, nil
}
