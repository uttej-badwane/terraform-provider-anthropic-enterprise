package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

var _ ephemeral.EphemeralResource = &federationTokenEphemeralResource{}
var _ ephemeral.EphemeralResourceWithConfigure = &federationTokenEphemeralResource{}

func init() { registerEphemeralResource(NewFederationTokenEphemeralResource) }

type federationTokenEphemeralResource struct {
	client *client.Client
}

// NewFederationTokenEphemeralResource exchanges an OIDC assertion for a
// short-lived Anthropic token.
func NewFederationTokenEphemeralResource() ephemeral.EphemeralResource {
	return &federationTokenEphemeralResource{}
}

type federationTokenEphemeralModel struct {
	FederationRuleID types.String `tfsdk:"federation_rule_id"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	WorkspaceID      types.String `tfsdk:"workspace_id"`
	Assertion        types.String `tfsdk:"assertion"`

	AccessToken types.String `tfsdk:"access_token"`
	TokenType   types.String `tfsdk:"token_type"`
	ExpiresIn   types.Int64  `tfsdk:"expires_in"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	Scope       types.String `tfsdk:"scope"`
}

func (r *federationTokenEphemeralResource) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_token"
}

func (r *federationTokenEphemeralResource) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Exchanges an OIDC assertion for a short-lived Anthropic access token under a " +
			"federation rule, so a pipeline can call the API without holding a long-lived key.\n\n" +
			"Ephemeral resources are never written to state or to the plan, which is the point: the minted " +
			"token exists only for the duration of the operation that used it.\n\n" +
			"Needs no provider credential. The assertion is what authenticates the exchange, and the " +
			"provider deliberately sends no API key with it.\n\n" +
			"~> Requires Terraform 1.10 or later. Workload identity federation is in beta.",
		Attributes: map[string]schema.Attribute{
			"federation_rule_id": schema.StringAttribute{
				MarkdownDescription: "Federation rule to evaluate the assertion against (`fdrl_...`).",
				Required:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Organization the rule belongs to, as a UUID.",
				Required:            true,
			},
			"service_account_id": schema.StringAttribute{
				MarkdownDescription: "Service account the minted token acts as (`svac_...`).",
				Required:            true,
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Workspace to scope the token to (`wrkspc_...`), or the literal `default` " +
					"for the organization's default workspace. Required when the rule is enabled for more than " +
					"one workspace; when omitted the server selects the rule's only enabled workspace.",
				Optional: true,
			},
			"assertion": schema.StringAttribute{
				MarkdownDescription: "The OIDC JWT issued by your identity provider, at most 16 KiB. In a " +
					"pipeline this is the token the platform mints for the job, such as the GitHub Actions " +
					"OIDC token. An assertion carrying a `jti` claim can be exchanged only once.",
				Required:  true,
				Sensitive: true,
			},

			"access_token": schema.StringAttribute{
				MarkdownDescription: "The minted token, prefixed `sk-ant-oat01-`. Pass it to a second provider " +
					"instance as `oauth_token`, or to anything expecting a bearer token.",
				Computed:  true,
				Sensitive: true,
			},
			"token_type": schema.StringAttribute{
				MarkdownDescription: "Always `Bearer`.",
				Computed:            true,
			},
			"expires_in": schema.Int64Attribute{
				MarkdownDescription: "Seconds the token remains valid, set by the rule's `token_lifetime_seconds`.",
				Computed:            true,
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "When the token expires, as an RFC 3339 timestamp, computed from " +
					"`expires_in` at the moment of the exchange.",
				Computed: true,
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "The OAuth scope the matched rule granted, such as `workspace:inference`.",
				Computed:            true,
			},
		},
	}
}

func (r *federationTokenEphemeralResource) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Expected *client.Client.")
		return
	}
	r.client = c
}

func (r *federationTokenEphemeralResource) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var cfg federationTokenEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tok, err := r.client.ExchangeFederationToken(ctx, client.FederationTokenExchange{
		Assertion:        cfg.Assertion.ValueString(),
		FederationRuleID: cfg.FederationRuleID.ValueString(),
		OrganizationID:   cfg.OrganizationID.ValueString(),
		ServiceAccountID: cfg.ServiceAccountID.ValueString(),
		WorkspaceID:      cfg.WorkspaceID.ValueString(),
	})
	if err != nil {
		// A denied exchange is always the same opaque 401, whatever the cause,
		// so point at where the reason is actually recorded.
		apiErrorDiag(&resp.Diagnostics, "Error exchanging the federation assertion", err)
		resp.Diagnostics.AddWarning("Where to find the reason",
			"A denied exchange returns the same message whichever check failed, by design. The deny reason "+
				"is recorded against the attempt in the Claude Console under Workload identity, on the "+
				"authentication history tab.")
		return
	}

	cfg.AccessToken = types.StringValue(tok.AccessToken)
	cfg.TokenType = types.StringValue(tok.TokenType)
	cfg.ExpiresIn = types.Int64Value(tok.ExpiresIn)
	cfg.ExpiresAt = types.StringValue(time.Now().UTC().Add(time.Duration(tok.ExpiresIn) * time.Second).Format(time.RFC3339))
	cfg.Scope = types.StringValue(tok.Scope)

	resp.Diagnostics.Append(resp.Result.Set(ctx, &cfg)...)
}

// Renew is deliberately not implemented. Renewing would mean exchanging again,
// and an assertion carrying a jti claim is single-use, so a second exchange of
// the same JWT is rejected as a replay. A run long enough to outlive the token
// needs a rule with a longer token_lifetime_seconds, not a renewal here.
