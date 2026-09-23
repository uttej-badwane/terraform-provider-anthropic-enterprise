package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewFederationRuleDataSource) }

var (
	_ datasource.DataSourceWithConfigure        = &federationRuleDataSource{}
	_ datasource.DataSourceWithConfigValidators = &federationRuleDataSource{}
)

// NewFederationRuleDataSource returns the anthropic_federation_rule data source.
func NewFederationRuleDataSource() datasource.DataSource { return &federationRuleDataSource{} }

type federationRuleDataSource struct{ client *client.Client }

var attrTypesRuleDetail = map[string]attr.Type{
	"id":                        types.StringType,
	"name":                      types.StringType,
	"issuer_id":                 types.StringType,
	"issuer_name":               types.StringType,
	"oauth_scope":               types.StringType,
	"service_account_id":        types.StringType,
	"service_account_name":      types.StringType,
	"description":               types.StringType,
	"subject_prefix":            types.StringType,
	"claims":                    types.MapType{ElemType: types.StringType},
	"condition":                 types.StringType,
	"audience":                  types.StringType,
	"applies_to_all_workspaces": types.BoolType,
	"workspace_ids":             types.ListType{ElemType: types.StringType},
	"token_lifetime_seconds":    types.Int64Type,
	"created_at":                types.StringType,
	"archived_at":               types.StringType,
	"created_by_actor_id":       types.StringType,
	"updated_by_actor_id":       types.StringType,
	"archived_by_actor_id":      types.StringType,
}

func (d *federationRuleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_rule"
}

func (d *federationRuleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up one workload identity federation rule by id or name. Requires `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"id":                        schema.StringAttribute{MarkdownDescription: "Rule id (`fdrl_...`). Exactly one of `id` or `name` must be set.", Optional: true, Computed: true},
			"name":                      schema.StringAttribute{MarkdownDescription: "Slug name; only live (unarchived) rules are matched.", Optional: true, Computed: true},
			"issuer_id":                 dsString("Issuer id."),
			"issuer_name":               dsString("Issuer name."),
			"oauth_scope":               dsString("Scope of minted tokens."),
			"service_account_id":        dsString("Target service account id."),
			"service_account_name":      dsString("Target service account name."),
			"description":               dsString("Description."),
			"subject_prefix":            dsString("Matched `sub` prefix (exact unless it ends in `*`)."),
			"claims":                    schema.MapAttribute{MarkdownDescription: "Exact-match claims.", Computed: true, ElementType: types.StringType},
			"condition":                 dsString("CEL condition over claims."),
			"audience":                  dsString("Exact `aud` match."),
			"applies_to_all_workspaces": dsBool("Whether the rule is enabled for every workspace."),
			"workspace_ids":             dsStringList("Workspaces the rule is enabled for."),
			"token_lifetime_seconds":    dsInt64("Lifetime of minted tokens."),
			"created_at":                dsString("Creation timestamp."),
			"archived_at":               dsString("Archive timestamp; null while live."),
			"created_by_actor_id":       dsString("Id of the user or service account that created it."),
			"updated_by_actor_id":       dsString("Id of the user or service account that last updated it."),
			"archived_by_actor_id":      dsString("Id of the user or service account that archived it; null while live."),
		},
	}
}

func (d *federationRuleDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name"))}
}

func (d *federationRuleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredOAuth, &resp.Diagnostics)
}

func (d *federationRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var rule *client.FederationRule
	if !id.IsNull() {
		got, err := d.client.GetFederationRule(ctx, id.ValueString())
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error reading federation rule", err)
			return
		}
		rule = got
	} else {
		list, err := d.client.ListFederationRules(ctx, client.FederationRuleListOptions{})
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error listing federation rules", err)
			return
		}
		for i := range list {
			if list[i].Name == name.ValueString() {
				if rule != nil {
					resp.Diagnostics.AddError("Ambiguous federation rule name", fmt.Sprintf("more than one live rule is named %q; use id", name.ValueString()))
					return
				}
				rule = &list[i]
			}
		}
		if rule == nil {
			resp.Diagnostics.AddError("Federation rule not found", fmt.Sprintf("no live federation rule named %q", name.ValueString()))
			return
		}
	}
	ids := rule.WorkspaceIDs
	if ids == nil {
		ids = []string{}
	}
	wl, diags := types.ListValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(diags...)
	claims, diags := types.MapValueFrom(ctx, types.StringType, nonNilMap(rule.Match.Claims))
	resp.Diagnostics.Append(diags...)
	obj, diags := types.ObjectValue(attrTypesRuleDetail, map[string]attr.Value{
		"id":                        types.StringValue(rule.ID),
		"name":                      types.StringValue(rule.Name),
		"issuer_id":                 types.StringValue(rule.IssuerID),
		"issuer_name":               stringFromPtr(rule.IssuerName),
		"oauth_scope":               types.StringValue(rule.OAuthScope),
		"service_account_id":        types.StringValue(rule.Target.ServiceAccountID),
		"service_account_name":      stringFromPtr(rule.Target.ServiceAccountName),
		"description":               stringFromPtr(rule.Description),
		"subject_prefix":            stringFromPtr(rule.Match.SubjectPrefix),
		"claims":                    claims,
		"condition":                 stringFromPtr(rule.Match.Condition),
		"audience":                  stringFromPtr(rule.Match.Audience),
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
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
