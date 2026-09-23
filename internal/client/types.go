package client

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Opt is an optional, nullable JSON value for update requests. Use the
// `omitzero` struct tag so unset fields are omitted while Null sends "null".
type Opt[T any] struct {
	Set   bool
	Null  bool
	Value T
}

// Some returns a set, non-null Opt.
func Some[T any](v T) Opt[T] { return Opt[T]{Set: true, Value: v} }

// Null returns a set Opt that serializes as JSON null.
func Null[T any]() Opt[T] { return Opt[T]{Set: true, Null: true} }

// IsZero implements the omitzero contract.
func (o Opt[T]) IsZero() bool { return !o.Set }

// MarshalJSON implements json.Marshaler.
func (o Opt[T]) MarshalJSON() ([]byte, error) {
	if o.Null {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *Opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

// Organization is GET /v1/organizations/me.
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// User is an organization member.
type User struct {
	ID      string `json:"id"`
	AddedAt string `json:"added_at"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	Type    string `json:"type"`
}

// UserUpdate changes a member's organization role.
type UserUpdate struct {
	Role string `json:"role"`
}

// Invite is a pending or historical invitation.
type Invite struct {
	ID           string   `json:"id"`
	AcceptedAt   *string  `json:"accepted_at"`
	Email        string   `json:"email"`
	ExpiresAt    string   `json:"expires_at"`
	InvitedAt    string   `json:"invited_at"`
	RBACGroupIDs []string `json:"rbac_group_ids"`
	Role         string   `json:"role"`
	Status       string   `json:"status"`
	Type         string   `json:"type"`
}

// InviteCreate is the create body for invites.
type InviteCreate struct {
	Email        string   `json:"email"`
	Role         string   `json:"role"`
	RBACGroupIDs []string `json:"rbac_group_ids,omitempty"`
}

// AllowedGeos is the allowed_inference_geos union: either the literal
// string "unrestricted" or a list of geos.
type AllowedGeos struct {
	Unrestricted bool
	Geos         []string
}

// MarshalJSON implements json.Marshaler.
func (a AllowedGeos) MarshalJSON() ([]byte, error) {
	if a.Unrestricted {
		return json.Marshal("unrestricted")
	}
	if a.Geos == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal(a.Geos)
}

// UnmarshalJSON implements json.Unmarshaler.
func (a *AllowedGeos) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*a = AllowedGeos{Unrestricted: true}
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s != "unrestricted" {
			return fmt.Errorf("unexpected allowed_inference_geos value %q", s)
		}
		*a = AllowedGeos{Unrestricted: true}
		return nil
	}
	var geos []string
	if err := json.Unmarshal(b, &geos); err != nil {
		return err
	}
	*a = AllowedGeos{Geos: geos}
	return nil
}

// DataResidency is the workspace data residency block.
type DataResidency struct {
	AllowedInferenceGeos AllowedGeos `json:"allowed_inference_geos"`
	DefaultInferenceGeo  string      `json:"default_inference_geo"`
	WorkspaceGeo         string      `json:"workspace_geo"`
}

// DataResidencyParams is the request form of DataResidency.
type DataResidencyParams struct {
	AllowedInferenceGeos *AllowedGeos `json:"allowed_inference_geos,omitempty"`
	DefaultInferenceGeo  *string      `json:"default_inference_geo,omitempty"`
	WorkspaceGeo         *string      `json:"workspace_geo,omitempty"`
}

// Workspace isolates API keys, members, limits and (optionally) a CMEK.
type Workspace struct {
	ID            string            `json:"id"`
	ArchivedAt    *string           `json:"archived_at"`
	CompartmentID string            `json:"compartment_id"`
	CreatedAt     string            `json:"created_at"`
	DataResidency *DataResidency    `json:"data_residency"`
	DisplayColor  string            `json:"display_color"`
	ExternalKeyID *string           `json:"external_key_id"`
	Name          string            `json:"name"`
	Tags          map[string]string `json:"tags"`
	Type          string            `json:"type"`
}

// WorkspaceCreate is the create body for workspaces.
type WorkspaceCreate struct {
	Name          string               `json:"name"`
	DisplayColor  *string              `json:"display_color,omitempty"`
	ExternalKeyID *string              `json:"external_key_id,omitempty"`
	Tags          map[string]string    `json:"tags,omitempty"`
	DataResidency *DataResidencyParams `json:"data_residency,omitempty"`
}

// WorkspaceUpdate is the update body; a nil tag value removes that tag.
type WorkspaceUpdate struct {
	Name          *string              `json:"name,omitempty"`
	DisplayColor  *string              `json:"display_color,omitempty"`
	ExternalKeyID *string              `json:"external_key_id,omitempty"`
	Tags          map[string]*string   `json:"tags,omitempty"`
	DataResidency *DataResidencyParams `json:"data_residency,omitempty"`
}

// WorkspaceMember binds a user to a workspace with a role.
type WorkspaceMember struct {
	Type          string `json:"type"`
	UserID        string `json:"user_id"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceRole string `json:"workspace_role"`
}

// WorkspaceMemberAdd is the add body.
type WorkspaceMemberAdd struct {
	UserID        string `json:"user_id"`
	WorkspaceRole string `json:"workspace_role"`
}

// WorkspaceRoleUpdate changes a workspace role (members and service accounts).
type WorkspaceRoleUpdate struct {
	WorkspaceRole string `json:"workspace_role"`
}

// ServiceAccountWorkspaceMember binds a service account to a workspace.
type ServiceAccountWorkspaceMember struct {
	CreatedByActorID *string `json:"created_by_actor_id"`
	Implicit         *bool   `json:"implicit"`
	ServiceAccountID string  `json:"service_account_id"`
	Type             string  `json:"type"`
	WorkspaceID      string  `json:"workspace_id"`
	WorkspaceRole    string  `json:"workspace_role"`
}

// ServiceAccountWorkspaceMemberAdd is the add body on the workspace path.
type ServiceAccountWorkspaceMemberAdd struct {
	ServiceAccountID string `json:"service_account_id"`
	WorkspaceRole    string `json:"workspace_role"`
}

// ActorRef is api_key.created_by.
type ActorRef struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Principal is api_key.principal.
type Principal struct {
	Type             string  `json:"type"`
	UserID           *string `json:"user_id,omitempty"`
	ServiceAccountID *string `json:"service_account_id,omitempty"`
}

// KeyScope is api_key.scope.
type KeyScope struct {
	Type        string  `json:"type"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
}

// APIKey is an API key record; the secret is never returned.
type APIKey struct {
	ID             string     `json:"id"`
	CreatedAt      string     `json:"created_at"`
	CreatedBy      *ActorRef  `json:"created_by"`
	ExpiresAt      *string    `json:"expires_at"`
	Name           string     `json:"name"`
	PartialKeyHint *string    `json:"partial_key_hint"`
	Principal      *Principal `json:"principal"`
	Scope          KeyScope   `json:"scope"`
	Status         string     `json:"status"`
	Type           string     `json:"type"`
	WorkspaceID    *string    `json:"workspace_id"`
}

// APIKeyUpdate renames or changes the status of a key.
type APIKeyUpdate struct {
	Name   *string `json:"name,omitempty"`
	Status *string `json:"status,omitempty"`
}

// RateLimitValue is one limiter within a rate limit group.
type RateLimitValue struct {
	Type     string `json:"type"`
	Value    int64  `json:"value"`
	OrgLimit *int64 `json:"org_limit,omitempty"`
}

// RateLimit is an organization rate limit group.
type RateLimit struct {
	ID        string           `json:"id"`
	GroupType string           `json:"group_type"`
	Limits    []RateLimitValue `json:"limits"`
	Models    []string         `json:"models"`
	Type      string           `json:"type"`
}

// WorkspaceRateLimit is a workspace override of a rate limit group.
type WorkspaceRateLimit struct {
	GroupType   string           `json:"group_type"`
	Limits      []RateLimitValue `json:"limits"`
	Models      []string         `json:"models"`
	RateLimitID string           `json:"rate_limit_id"`
	Type        string           `json:"type"`
	WorkspaceID string           `json:"workspace_id"`
}

// ServiceAccount is a non-human principal.
type ServiceAccount struct {
	ID                string  `json:"id"`
	ArchivedAt        *string `json:"archived_at"`
	ArchivedByActorID *string `json:"archived_by_actor_id"`
	CreatedAt         string  `json:"created_at"`
	CreatedByActorID  *string `json:"created_by_actor_id"`
	Description       *string `json:"description"`
	Name              string  `json:"name"`
	OrganizationRole  string  `json:"organization_role"`
	Type              string  `json:"type"`
	UpdatedAt         string  `json:"updated_at"`
	UpdatedByActorID  *string `json:"updated_by_actor_id"`
}

// ServiceAccountCreate is the create body.
type ServiceAccountCreate struct {
	Name             string  `json:"name"`
	Description      *string `json:"description,omitempty"`
	OrganizationRole *string `json:"organization_role,omitempty"`
}

// ServiceAccountUpdate is the update body; name is immutable.
type ServiceAccountUpdate struct {
	Description      Opt[string] `json:"description,omitzero"`
	OrganizationRole *string     `json:"organization_role,omitempty"`
}

// JWKS is the federation issuer key-source union.
type JWKS struct {
	Type          string           `json:"type"`
	CACertPEM     *string          `json:"ca_cert_pem,omitempty"`
	DiscoveryBase *string          `json:"discovery_base,omitempty"`
	URL           *string          `json:"url,omitempty"`
	Keys          []map[string]any `json:"keys,omitempty"`
}

// PollStatus is the issuer's JWKS polling state.
type PollStatus struct {
	ConsecutiveFailures int64   `json:"consecutive_failures"`
	LastFetchedAt       *string `json:"last_fetched_at"`
	NextPollAt          *string `json:"next_poll_at"`
}

// FederationIssuer is a trusted OIDC token issuer.
type FederationIssuer struct {
	ID                    string      `json:"id"`
	ArchivedAt            *string     `json:"archived_at"`
	ArchivedByActorID     *string     `json:"archived_by_actor_id"`
	CheckJTI              bool        `json:"check_jti"`
	CreatedAt             string      `json:"created_at"`
	CreatedByActorID      *string     `json:"created_by_actor_id"`
	IssuerURL             string      `json:"issuer_url"`
	JWKS                  JWKS        `json:"jwks"`
	JWKSPollingDisabledAt *string     `json:"jwks_polling_disabled_at"`
	MaxJWTLifetimeSeconds int64       `json:"max_jwt_lifetime_seconds"`
	Name                  string      `json:"name"`
	PollStatus            *PollStatus `json:"poll_status"`
	Type                  string      `json:"type"`
	UpdatedAt             string      `json:"updated_at"`
	UpdatedByActorID      *string     `json:"updated_by_actor_id"`
}

// FederationIssuerCreate is the create body.
type FederationIssuerCreate struct {
	IssuerURL             string `json:"issuer_url"`
	Name                  string `json:"name"`
	CheckJTI              *bool  `json:"check_jti,omitempty"`
	JWKS                  *JWKS  `json:"jwks,omitempty"`
	MaxJWTLifetimeSeconds *int64 `json:"max_jwt_lifetime_seconds,omitempty"`
}

// FederationIssuerUpdate is the update body.
type FederationIssuerUpdate struct {
	CheckJTI              *bool   `json:"check_jti,omitempty"`
	IssuerURL             *string `json:"issuer_url,omitempty"`
	JWKS                  *JWKS   `json:"jwks,omitempty"`
	JWKSPollingDisabled   *bool   `json:"jwks_polling_disabled,omitempty"`
	MaxJWTLifetimeSeconds *int64  `json:"max_jwt_lifetime_seconds,omitempty"`
	Name                  *string `json:"name,omitempty"`
}

// RuleMatch selects which tokens a federation rule accepts.
type RuleMatch struct {
	Audience      *string           `json:"audience,omitempty"`
	Claims        map[string]string `json:"claims,omitempty"`
	Condition     *string           `json:"condition,omitempty"`
	SubjectPrefix *string           `json:"subject_prefix,omitempty"`
}

// RuleTarget is the principal a federation rule mints tokens for.
type RuleTarget struct {
	Type               string  `json:"type"`
	ServiceAccountID   string  `json:"service_account_id"`
	ServiceAccountName *string `json:"service_account_name,omitempty"`
}

// FederationRule maps issuer tokens to a service account.
type FederationRule struct {
	ID                     string            `json:"id"`
	AppliesToAllWorkspaces bool              `json:"applies_to_all_workspaces"`
	ArchivedAt             *string           `json:"archived_at"`
	ArchivedByActorID      *string           `json:"archived_by_actor_id"`
	Attributes             map[string]string `json:"attributes"`
	CreatedAt              string            `json:"created_at"`
	CreatedByActorID       *string           `json:"created_by_actor_id"`
	Description            *string           `json:"description"`
	IssuerID               string            `json:"issuer_id"`
	IssuerName             *string           `json:"issuer_name"`
	Match                  RuleMatch         `json:"match"`
	Name                   string            `json:"name"`
	OAuthScope             string            `json:"oauth_scope"`
	Target                 RuleTarget        `json:"target"`
	TokenLifetimeSeconds   int64             `json:"token_lifetime_seconds"`
	Type                   string            `json:"type"`
	UpdatedAt              string            `json:"updated_at"`
	UpdatedByActorID       *string           `json:"updated_by_actor_id"`
	WorkspaceID            *string           `json:"workspace_id"`
	WorkspaceIDs           []string          `json:"workspace_ids"`
}

// FederationRuleCreate is the create body.
type FederationRuleCreate struct {
	IssuerID               string            `json:"issuer_id"`
	Match                  RuleMatch         `json:"match"`
	Name                   string            `json:"name"`
	OAuthScope             string            `json:"oauth_scope"`
	Target                 RuleTarget        `json:"target"`
	AppliesToAllWorkspaces *bool             `json:"applies_to_all_workspaces,omitempty"`
	Attributes             map[string]string `json:"attributes,omitempty"`
	Description            *string           `json:"description,omitempty"`
	TokenLifetimeSeconds   *int64            `json:"token_lifetime_seconds,omitempty"`
	WorkspaceID            *string           `json:"workspace_id,omitempty"`
}

// FederationRuleUpdate is the update body.
type FederationRuleUpdate struct {
	AppliesToAllWorkspaces *bool       `json:"applies_to_all_workspaces,omitempty"`
	Description            Opt[string] `json:"description,omitzero"`
	Match                  *RuleMatch  `json:"match,omitempty"`
	Name                   *string     `json:"name,omitempty"`
	OAuthScope             *string     `json:"oauth_scope,omitempty"`
	Target                 *RuleTarget `json:"target,omitempty"`
	TokenLifetimeSeconds   *int64      `json:"token_lifetime_seconds,omitempty"`
	WorkspaceID            *string     `json:"workspace_id,omitempty"`
}

// FederationRuleWorkspace is a rule-to-workspace binding.
type FederationRuleWorkspace struct {
	CreatedAt        string  `json:"created_at"`
	CreatedByActorID *string `json:"created_by_actor_id"`
	FederationRuleID string  `json:"federation_rule_id"`
	Type             string  `json:"type"`
	WorkspaceID      string  `json:"workspace_id"`
	WorkspaceName    *string `json:"workspace_name"`
}

// ProviderConfig is the external key KMS configuration union.
type ProviderConfig struct {
	Type     string  `json:"type"`
	KMSARN   *string `json:"kms_arn,omitempty"`
	Region   *string `json:"region,omitempty"`
	RoleARN  *string `json:"role_arn,omitempty"`
	KeyName  *string `json:"key_name,omitempty"`
	TenantID *string `json:"tenant_id,omitempty"`
	VaultURI *string `json:"vault_uri,omitempty"`
	ClientID *string `json:"client_id,omitempty"`
}

// Attachment is external_key.attachment.
type Attachment struct {
	Type string `json:"type"`
}

// ExternalKey is a customer-managed encryption key registration.
type ExternalKey struct {
	ID             string         `json:"id"`
	Attachment     Attachment     `json:"attachment"`
	CreatedAt      string         `json:"created_at"`
	DisplayName    *string        `json:"display_name"`
	Geo            string         `json:"geo"`
	ProviderConfig ProviderConfig `json:"provider_config"`
	Type           string         `json:"type"`
	UpdatedAt      string         `json:"updated_at"`
}

// ExternalKeyCreate is the create body.
type ExternalKeyCreate struct {
	ProviderConfig ProviderConfig `json:"provider_config"`
	DisplayName    *string        `json:"display_name,omitempty"`
	Geo            *string        `json:"geo,omitempty"`
}

// ExternalKeyUpdate is the update body.
type ExternalKeyUpdate struct {
	DisplayName    Opt[string]     `json:"display_name,omitzero"`
	Geo            *string         `json:"geo,omitempty"`
	ProviderConfig *ProviderConfig `json:"provider_config,omitempty"`
}

// ExternalKeyValidation is the validate response.
type ExternalKeyValidation struct {
	Error  *string `json:"error"`
	Status string  `json:"status"`
	Type   string  `json:"type"`
}

// RBACGroup is a Claude Enterprise group.
type RBACGroup struct {
	ID         string   `json:"id"`
	CreatedAt  string   `json:"created_at"`
	Name       string   `json:"name"`
	Roles      []string `json:"roles"`
	SourceType string   `json:"source_type"`
	Type       string   `json:"type"`
	UpdatedAt  string   `json:"updated_at"`
}

// RBACGroupWrite is the create/update body.
type RBACGroupWrite struct {
	Name string `json:"name"`
}

// RBACGroupMember is a user's membership in a group.
type RBACGroupMember struct {
	CreatedAt string `json:"created_at"`
	Email     string `json:"email"`
	GroupID   string `json:"group_id"`
	Type      string `json:"type"`
	UserID    string `json:"user_id"`
}

// RBACGroupMemberAdd is the add body.
type RBACGroupMemberAdd struct {
	UserID string `json:"user_id"`
}

// RBACRole is a Claude Enterprise role.
type RBACRole struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	UpdatedAt string `json:"updated_at"`
}

// RBACRolePermission is one permission of a role.
type RBACRolePermission struct {
	Action   string         `json:"action"`
	Resource map[string]any `json:"resource"`
	Type     string         `json:"type"`
}

// SpendScope is the spend limit scope/source union.
type SpendScope struct {
	Type        string  `json:"type"`
	UserID      *string `json:"user_id,omitempty"`
	SeatTier    *string `json:"seat_tier,omitempty"`
	RBACGroupID *string `json:"rbac_group_id,omitempty"`
	Service     *string `json:"service,omitempty"`
}

// SpendLimit is a per-scope spending cap.
type SpendLimit struct {
	ID        string     `json:"id"`
	Amount    *string    `json:"amount"`
	CreatedAt string     `json:"created_at"`
	Currency  string     `json:"currency"`
	Period    string     `json:"period"`
	Scope     SpendScope `json:"scope"`
	Type      string     `json:"type"`
	UpdatedAt string     `json:"updated_at"`
}

// SpendLimitCreate upserts a per-user limit. Amount is always sent; nil means
// "no numeric cap".
type SpendLimitCreate struct {
	Amount *string    `json:"amount"`
	Scope  SpendScope `json:"scope"`
	Period *string    `json:"period,omitempty"`
}

// SpendActor is the actor in an effective spend summary.
type SpendActor struct {
	Deleted      bool    `json:"deleted"`
	EmailAddress *string `json:"email_address"`
	Name         *string `json:"name"`
	Type         string  `json:"type"`
	UserID       string  `json:"user_id"`
}

// SpendSummary is one row of GET /spend_limits/effective.
type SpendSummary struct {
	Actor             SpendActor `json:"actor"`
	Amount            *string    `json:"amount"`
	Currency          string     `json:"currency"`
	Period            string     `json:"period"`
	PeriodToDateSpend string     `json:"period_to_date_spend"`
	Scope             SpendScope `json:"scope"`
	Source            SpendScope `json:"source"`
	SpendLimitID      string     `json:"spend_limit_id"`
}

// ComplianceState is compliance_settings.state.
type ComplianceState struct {
	Type string `json:"type"`
}

// ComplianceSettings is the organization compliance API toggle.
type ComplianceSettings struct {
	State ComplianceState `json:"state"`
	Type  string          `json:"type"`
}

// ComplianceSettingsUpdate is the update body.
type ComplianceSettingsUpdate struct {
	State ComplianceState `json:"state"`
}
