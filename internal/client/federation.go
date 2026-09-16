package client

import (
	"context"
	"net/url"
)

func serviceAccountPath(id string) string { return orgPath + "/service_accounts/" + url.PathEscape(id) }
func issuerPath(id string) string         { return orgPath + "/federation_issuers/" + url.PathEscape(id) }
func rulePath(id string) string           { return orgPath + "/federation_rules/" + url.PathEscape(id) }

func includeArchivedQuery(v bool) url.Values {
	q := url.Values{}
	if v {
		q.Set("include_archived", "true")
	}
	return q
}

// --- service accounts (OAuth only) -----------------------------------------

// ListServiceAccounts lists service accounts.
func (c *Client) ListServiceAccounts(ctx context.Context, includeArchived bool) ([]ServiceAccount, error) {
	return listToken[ServiceAccount](ctx, c, CredOAuth, orgPath+"/service_accounts", includeArchivedQuery(includeArchived))
}

// CreateServiceAccount creates a service account.
func (c *Client) CreateServiceAccount(ctx context.Context, in ServiceAccountCreate) (*ServiceAccount, error) {
	var out ServiceAccount
	if err := c.create(ctx, CredOAuth, orgPath+"/service_accounts", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetServiceAccount returns one service account.
func (c *Client) GetServiceAccount(ctx context.Context, id string) (*ServiceAccount, error) {
	var out ServiceAccount
	if err := c.get(ctx, CredOAuth, serviceAccountPath(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateServiceAccount updates description/role.
func (c *Client) UpdateServiceAccount(ctx context.Context, id string, in ServiceAccountUpdate) (*ServiceAccount, error) {
	var out ServiceAccount
	if err := c.post(ctx, CredOAuth, serviceAccountPath(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveServiceAccount archives a service account (irreversible).
func (c *Client) ArchiveServiceAccount(ctx context.Context, id string) (*ServiceAccount, error) {
	var out ServiceAccount
	if err := c.post(ctx, CredOAuth, serviceAccountPath(id)+"/archive", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- federation issuers (OAuth only) ---------------------------------------

// ListFederationIssuers lists issuers.
func (c *Client) ListFederationIssuers(ctx context.Context, includeArchived bool) ([]FederationIssuer, error) {
	return listToken[FederationIssuer](ctx, c, CredOAuth, orgPath+"/federation_issuers", includeArchivedQuery(includeArchived))
}

// CreateFederationIssuer registers an OIDC issuer.
func (c *Client) CreateFederationIssuer(ctx context.Context, in FederationIssuerCreate) (*FederationIssuer, error) {
	var out FederationIssuer
	if err := c.create(ctx, CredOAuth, orgPath+"/federation_issuers", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFederationIssuer returns one issuer.
func (c *Client) GetFederationIssuer(ctx context.Context, id string) (*FederationIssuer, error) {
	var out FederationIssuer
	if err := c.get(ctx, CredOAuth, issuerPath(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateFederationIssuer updates an issuer.
func (c *Client) UpdateFederationIssuer(ctx context.Context, id string, in FederationIssuerUpdate) (*FederationIssuer, error) {
	var out FederationIssuer
	if err := c.post(ctx, CredOAuth, issuerPath(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveFederationIssuer archives an issuer (irreversible).
func (c *Client) ArchiveFederationIssuer(ctx context.Context, id string) (*FederationIssuer, error) {
	var out FederationIssuer
	if err := c.post(ctx, CredOAuth, issuerPath(id)+"/archive", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- federation rules (OAuth only) -----------------------------------------

// FederationRuleListOptions filters ListFederationRules.
type FederationRuleListOptions struct {
	IncludeArchived bool
	IssuerID        string
}

// ListFederationRules lists rules.
func (c *Client) ListFederationRules(ctx context.Context, opts FederationRuleListOptions) ([]FederationRule, error) {
	q := includeArchivedQuery(opts.IncludeArchived)
	if opts.IssuerID != "" {
		q.Set("issuer_id", opts.IssuerID)
	}
	return listToken[FederationRule](ctx, c, CredOAuth, orgPath+"/federation_rules", q)
}

// CreateFederationRule creates a rule.
func (c *Client) CreateFederationRule(ctx context.Context, in FederationRuleCreate) (*FederationRule, error) {
	var out FederationRule
	if err := c.create(ctx, CredOAuth, orgPath+"/federation_rules", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFederationRule returns one rule.
func (c *Client) GetFederationRule(ctx context.Context, id string) (*FederationRule, error) {
	var out FederationRule
	if err := c.get(ctx, CredOAuth, rulePath(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateFederationRule updates a rule.
func (c *Client) UpdateFederationRule(ctx context.Context, id string, in FederationRuleUpdate) (*FederationRule, error) {
	var out FederationRule
	if err := c.post(ctx, CredOAuth, rulePath(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveFederationRule archives a rule (irreversible).
func (c *Client) ArchiveFederationRule(ctx context.Context, id string) (*FederationRule, error) {
	var out FederationRule
	if err := c.post(ctx, CredOAuth, rulePath(id)+"/archive", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListFederationRuleWorkspaces lists the workspaces a rule is enabled for.
func (c *Client) ListFederationRuleWorkspaces(ctx context.Context, ruleID string) ([]FederationRuleWorkspace, error) {
	return listToken[FederationRuleWorkspace](ctx, c, CredOAuth, rulePath(ruleID)+"/workspaces", nil)
}

// AddFederationRuleWorkspace enables a rule for a workspace.
func (c *Client) AddFederationRuleWorkspace(ctx context.Context, ruleID, workspaceID string) (*FederationRuleWorkspace, error) {
	var out FederationRuleWorkspace
	body := map[string]string{"workspace_id": workspaceID}
	if err := c.create(ctx, CredOAuth, rulePath(ruleID)+"/workspaces", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveFederationRuleWorkspace disables a rule for a workspace.
func (c *Client) RemoveFederationRuleWorkspace(ctx context.Context, ruleID, workspaceID string) error {
	return c.delete(ctx, CredOAuth, rulePath(ruleID)+"/workspaces/"+url.PathEscape(workspaceID))
}

// --- external keys (Admin) -------------------------------------------------

func externalKeyPath(id string) string { return orgPath + "/external_keys/" + url.PathEscape(id) }

// ListExternalKeys lists CMEK registrations.
func (c *Client) ListExternalKeys(ctx context.Context) ([]ExternalKey, error) {
	return listToken[ExternalKey](ctx, c, CredAdmin, orgPath+"/external_keys", nil)
}

// CreateExternalKey registers a CMEK.
func (c *Client) CreateExternalKey(ctx context.Context, in ExternalKeyCreate) (*ExternalKey, error) {
	var out ExternalKey
	if err := c.create(ctx, CredAdmin, orgPath+"/external_keys", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetExternalKey returns one CMEK registration.
func (c *Client) GetExternalKey(ctx context.Context, id string) (*ExternalKey, error) {
	var out ExternalKey
	if err := c.get(ctx, CredAdmin, externalKeyPath(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateExternalKey updates a CMEK registration.
func (c *Client) UpdateExternalKey(ctx context.Context, id string, in ExternalKeyUpdate) (*ExternalKey, error) {
	var out ExternalKey
	if err := c.post(ctx, CredAdmin, externalKeyPath(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteExternalKey deletes an unattached CMEK registration.
func (c *Client) DeleteExternalKey(ctx context.Context, id string) error {
	return c.delete(ctx, CredAdmin, externalKeyPath(id))
}

// ValidateExternalKey checks that the KMS key is reachable.
func (c *Client) ValidateExternalKey(ctx context.Context, id string) (*ExternalKeyValidation, error) {
	var out ExternalKeyValidation
	if err := c.post(ctx, CredAdmin, externalKeyPath(id)+"/validate", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
