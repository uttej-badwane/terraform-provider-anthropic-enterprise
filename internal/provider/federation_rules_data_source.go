package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewFederationRulesDataSource) }

var _ datasource.DataSourceWithConfigure = &federationRulesDataSource{}

// NewFederationRulesDataSource returns the anthropic_federation_rules data source.
func NewFederationRulesDataSource() datasource.DataSource { return &federationRulesDataSource{} }

type federationRulesDataSource struct{ client *client.Client }

type federationRulesModel struct {
	IncludeArchived types.Bool   `tfsdk:"include_archived"`
	IssuerID        types.String `tfsdk:"issuer_id"`
	Rules           types.List   `tfsdk:"rules"`
}

var attrTypesRuleSummary = map[string]attr.Type{
	"id":                        types.StringType,
	"name":                      types.StringType,
	"issuer_id":                 types.StringType,
	"oauth_scope":               types.StringType,
	"service_account_id":        types.StringType,
	"applies_to_all_workspaces": types.BoolType,
	"workspace_ids":             types.ListType{ElemType: types.StringType},
	"token_lifetime_seconds":    types.Int64Type,
	"created_at":                types.StringType,
	"archived_at":               types.StringType,
	"created_by_actor_id":       types.StringType,
	"updated_by_actor_id":       types.StringType,
	"archived_by_actor_id":      types.StringType,
}

func (d *federationRulesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_rules"
}

func (d *federationRulesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists workload identity federation rules. Requires `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"include_archived": schema.BoolAttribute{MarkdownDescription: "Include archived rules. Defaults to `false`.", Optional: true},
			"issuer_id":        schema.StringAttribute{MarkdownDescription: "Only return rules for this issuer.", Optional: true},
			"rules": schema.ListNestedAttribute{
				MarkdownDescription: "Rules.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":                        dsString("Rule id."),
					"name":                      dsString("Slug name."),
					"issuer_id":                 dsString("Issuer id."),
					"oauth_scope":               dsString("Scope of minted tokens."),
					"service_account_id":        dsString("Target service account id."),
					"applies_to_all_workspaces": dsBool("Whether the rule is enabled for every workspace."),
					"workspace_ids":             dsStringList("Workspaces the rule is enabled for."),
					"token_lifetime_seconds":    dsInt64("Lifetime of minted tokens."),
					"created_at":                dsString("Creation timestamp."),
					"archived_at":               dsString("Archive timestamp; null while live."),
					"created_by_actor_id":       dsString("Id of the user or service account that created it."),
					"updated_by_actor_id":       dsString("Id of the user or service account that last updated it."),
					"archived_by_actor_id":      dsString("Id of the user or service account that archived it; null while live."),
				}},
			},
		},
	}
}

func (d *federationRulesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredOAuth, &resp.Diagnostics)
}

func (d *federationRulesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg federationRulesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListFederationRules(ctx, client.FederationRuleListOptions{
		IncludeArchived: cfg.IncludeArchived.ValueBool(),
		IssuerID:        cfg.IssuerID.ValueString(),
	})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing federation rules", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for _, rule := range list {
		ids := rule.WorkspaceIDs
		if ids == nil {
			ids = []string{}
		}
		wl, diags := types.ListValueFrom(ctx, types.StringType, ids)
		resp.Diagnostics.Append(diags...)
		obj, diags := types.ObjectValue(attrTypesRuleSummary, map[string]attr.Value{
			"id":                        types.StringValue(rule.ID),
			"name":                      types.StringValue(rule.Name),
			"issuer_id":                 types.StringValue(rule.IssuerID),
			"oauth_scope":               types.StringValue(rule.OAuthScope),
			"service_account_id":        types.StringValue(rule.Target.ServiceAccountID),
			"applies_to_all_workspaces": types.BoolValue(rule.AppliesToAllWorkspaces),
			"workspace_ids":             wl,
			"token_lifetime_seconds":    types.Int64Value(rule.TokenLifetimeSeconds),
			"created_at":                types.StringValue(rule.CreatedAt),
			"archived_at":               stringFromPtr(rule.ArchivedAt),
			"created_by_actor_id":       stringFromPtr(rule.CreatedByActorID),
			"updated_by_actor_id":       stringFromPtr(rule.UpdatedByActorID),
			"archived_by_actor_id":      stringFromPtr(rule.ArchivedByActorID),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesRuleSummary}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.Rules = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
