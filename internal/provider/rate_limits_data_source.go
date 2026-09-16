package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewRateLimitsDataSource)
	registerDataSource(NewWorkspaceRateLimitsDataSource)
}

var rateLimitValueType = types.ObjectType{AttrTypes: map[string]attr.Type{"type": types.StringType, "value": types.Int64Type}}
var workspaceRateLimitValueType = types.ObjectType{AttrTypes: map[string]attr.Type{"type": types.StringType, "value": types.Int64Type, "org_limit": types.Int64Type}}

var attrTypesRateLimit = map[string]attr.Type{
	"id":         types.StringType,
	"group_type": types.StringType,
	"models":     types.ListType{ElemType: types.StringType},
	"limits":     types.ListType{ElemType: rateLimitValueType},
}

var attrTypesWorkspaceRateLimit = map[string]attr.Type{
	"rate_limit_id": types.StringType,
	"group_type":    types.StringType,
	"models":        types.ListType{ElemType: types.StringType},
	"limits":        types.ListType{ElemType: workspaceRateLimitValueType},
}

func limitsList(vals []client.RateLimitValue, withOrg bool) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := rateLimitValueType
	if withOrg {
		elemType = workspaceRateLimitValueType
	}
	objs := make([]attr.Value, 0, len(vals))
	for _, v := range vals {
		m := map[string]attr.Value{"type": types.StringValue(v.Type), "value": types.Int64Value(v.Value)}
		if withOrg {
			m["org_limit"] = int64FromPtr(v.OrgLimit)
		}
		obj, d := types.ObjectValue(elemType.AttrTypes, m)
		diags.Append(d...)
		objs = append(objs, obj)
	}
	l, d := types.ListValue(elemType, objs)
	diags.Append(d...)
	return l, diags
}

// --- anthropic_rate_limits --------------------------------------------------

var _ datasource.DataSourceWithConfigure = &rateLimitsDataSource{}

// NewRateLimitsDataSource returns the anthropic_rate_limits data source.
func NewRateLimitsDataSource() datasource.DataSource { return &rateLimitsDataSource{} }

type rateLimitsDataSource struct{ client *client.Client }

type rateLimitsModel struct {
	Model      types.String `tfsdk:"model"`
	GroupType  types.String `tfsdk:"group_type"`
	RateLimits types.List   `tfsdk:"rate_limits"`
}

func (d *rateLimitsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rate_limits"
}

func (d *rateLimitsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Organization rate limit groups. Read-only: limits cannot be changed through the API. Requires `admin_api_key` or `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"model":      schema.StringAttribute{MarkdownDescription: "Filter to the group containing this model name or alias. Unknown models return an error.", Optional: true},
			"group_type": schema.StringAttribute{MarkdownDescription: "Filter by group type: `batch`, `files`, `model_group`, `skills`, `token_count`, `web_search`.", Optional: true},
			"rate_limits": schema.ListNestedAttribute{
				MarkdownDescription: "Rate limit groups.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":         dsString("Rate limit group id."),
					"group_type": dsString("Group type."),
					"models":     dsStringList("Models in the group; null unless `group_type` is `model_group`."),
					"limits": schema.ListNestedAttribute{
						MarkdownDescription: "Limiters in the group.",
						Computed:            true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"type":  dsString("Limiter type, for example `requests_per_minute`."),
							"value": dsInt64("Limit value."),
						}},
					},
				}},
			},
		},
	}
}

func (d *rateLimitsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *rateLimitsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg rateLimitsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListRateLimits(ctx, client.RateLimitListOptions{Model: cfg.Model.ValueString(), GroupType: cfg.GroupType.ValueString()})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing rate limits", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for _, rl := range list {
		models, d1 := stringList(ctx, rl.Models)
		limits, d2 := limitsList(rl.Limits, false)
		resp.Diagnostics.Append(d1...)
		resp.Diagnostics.Append(d2...)
		obj, d3 := types.ObjectValue(attrTypesRateLimit, map[string]attr.Value{
			"id": types.StringValue(rl.ID), "group_type": types.StringValue(rl.GroupType), "models": models, "limits": limits,
		})
		resp.Diagnostics.Append(d3...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesRateLimit}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.RateLimits = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_workspace_rate_limits ----------------------------------------

var _ datasource.DataSourceWithConfigure = &workspaceRateLimitsDataSource{}

// NewWorkspaceRateLimitsDataSource returns the anthropic_workspace_rate_limits data source.
func NewWorkspaceRateLimitsDataSource() datasource.DataSource {
	return &workspaceRateLimitsDataSource{}
}

type workspaceRateLimitsDataSource struct{ client *client.Client }

type workspaceRateLimitsModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	GroupType   types.String `tfsdk:"group_type"`
	RateLimits  types.List   `tfsdk:"rate_limits"`
}

func (d *workspaceRateLimitsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_rate_limits"
}

func (d *workspaceRateLimitsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Rate limit overrides configured for a workspace. Only limiters that override the organization value are returned. Read-only. Requires `admin_api_key` or `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{MarkdownDescription: "Workspace id.", Required: true},
			"group_type":   schema.StringAttribute{MarkdownDescription: "Filter by group type.", Optional: true},
			"rate_limits": schema.ListNestedAttribute{
				MarkdownDescription: "Workspace rate limit overrides.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"rate_limit_id": dsString("Id of the organization rate limit group."),
					"group_type":    dsString("Group type."),
					"models":        dsStringList("Models in the group, if any."),
					"limits": schema.ListNestedAttribute{
						MarkdownDescription: "Overridden limiters.",
						Computed:            true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"type":      dsString("Limiter type."),
							"value":     dsInt64("Workspace limit value."),
							"org_limit": dsInt64("Organization limit value, if reported."),
						}},
					},
				}},
			},
		},
	}
}

func (d *workspaceRateLimitsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *workspaceRateLimitsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg workspaceRateLimitsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListWorkspaceRateLimits(ctx, cfg.WorkspaceID.ValueString(), cfg.GroupType.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing workspace rate limits", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for _, rl := range list {
		models, d1 := stringList(ctx, rl.Models)
		limits, d2 := limitsList(rl.Limits, true)
		resp.Diagnostics.Append(d1...)
		resp.Diagnostics.Append(d2...)
		obj, d3 := types.ObjectValue(attrTypesWorkspaceRateLimit, map[string]attr.Value{
			"rate_limit_id": types.StringValue(rl.RateLimitID), "group_type": types.StringValue(rl.GroupType), "models": models, "limits": limits,
		})
		resp.Diagnostics.Append(d3...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesWorkspaceRateLimit}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.RateLimits = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
