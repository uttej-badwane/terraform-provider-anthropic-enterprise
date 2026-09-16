package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewInvitesDataSource)
	registerDataSource(NewInviteDataSource)
}

var attrTypesInvite = map[string]attr.Type{
	"id":             types.StringType,
	"email":          types.StringType,
	"role":           types.StringType,
	"status":         types.StringType,
	"invited_at":     types.StringType,
	"expires_at":     types.StringType,
	"accepted_at":    types.StringType,
	"rbac_group_ids": types.ListType{ElemType: types.StringType},
}

var inviteDSAttributes = map[string]schema.Attribute{
	"id":             dsString("Invite id."),
	"email":          dsString("Invited email address."),
	"role":           dsString("Organization role granted on acceptance."),
	"status":         dsString("`pending`, `accepted`, `expired` or `deleted`."),
	"invited_at":     dsString("Timestamp the invite was sent."),
	"expires_at":     dsString("Timestamp the invite expires."),
	"accepted_at":    dsString("Timestamp the invite was accepted; null until then."),
	"rbac_group_ids": dsStringList("Claude Enterprise RBAC group ids assigned on acceptance."),
}

func inviteObject(ctx context.Context, inv *client.Invite) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	ids := inv.RBACGroupIDs
	if ids == nil {
		ids = []string{}
	}
	groups, d := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	obj, d := types.ObjectValue(attrTypesInvite, map[string]attr.Value{
		"id":             types.StringValue(inv.ID),
		"email":          types.StringValue(inv.Email),
		"role":           types.StringValue(inv.Role),
		"status":         types.StringValue(inv.Status),
		"invited_at":     types.StringValue(inv.InvitedAt),
		"expires_at":     types.StringValue(inv.ExpiresAt),
		"accepted_at":    stringFromPtr(inv.AcceptedAt),
		"rbac_group_ids": groups,
	})
	diags.Append(d...)
	return obj, diags
}

// --- anthropic_invites ------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &invitesDataSource{}

// NewInvitesDataSource returns the anthropic_invites data source.
func NewInvitesDataSource() datasource.DataSource { return &invitesDataSource{} }

type invitesDataSource struct{ client *client.Client }

type invitesModel struct {
	Email    types.String `tfsdk:"email"`
	Roles    types.List   `tfsdk:"roles"`
	Statuses types.List   `tfsdk:"statuses"`
	Invites  types.List   `tfsdk:"invites"`
}

func (d *invitesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_invites"
}

func (d *invitesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists organization invitations. Works with any admin or enterprise credential.",
		Attributes: map[string]schema.Attribute{
			"email":    schema.StringAttribute{MarkdownDescription: "Filter by invited email address.", Optional: true},
			"roles":    schema.ListAttribute{MarkdownDescription: "Filter by organization roles (any match).", Optional: true, ElementType: types.StringType},
			"statuses": schema.ListAttribute{MarkdownDescription: "Filter by status (`pending`, `accepted`, `expired`). Defaults to all.", Optional: true, ElementType: types.StringType},
			"invites": schema.ListNestedAttribute{
				MarkdownDescription: "Invitations.",
				Computed:            true,
				NestedObject:        schema.NestedAttributeObject{Attributes: inviteDSAttributes},
			},
		},
	}
}

func (d *invitesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = memberClientFromDataSource(req, &resp.Diagnostics)
}

func (d *invitesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg invitesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opts := client.InviteListOptions{Email: cfg.Email.ValueString()}
	if !cfg.Roles.IsNull() {
		resp.Diagnostics.Append(cfg.Roles.ElementsAs(ctx, &opts.Roles, false)...)
	}
	if !cfg.Statuses.IsNull() {
		resp.Diagnostics.Append(cfg.Statuses.ElementsAs(ctx, &opts.Statuses, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListInvites(ctx, opts)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing invites", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for i := range list {
		obj, diags := inviteObject(ctx, &list[i])
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesInvite}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.Invites = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_invite -------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &inviteDataSource{}

// NewInviteDataSource returns the anthropic_invite data source.
func NewInviteDataSource() datasource.DataSource { return &inviteDataSource{} }

type inviteDataSource struct{ client *client.Client }

func (d *inviteDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_invite"
}

func (d *inviteDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{}
	for k, v := range inviteDSAttributes {
		attrs[k] = v
	}
	attrs["id"] = schema.StringAttribute{MarkdownDescription: "Invite id (`invite_...`).", Required: true}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one organization invitation by id. Works with any admin or enterprise credential.",
		Attributes:          attrs,
	}
}

func (d *inviteDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = memberClientFromDataSource(req, &resp.Diagnostics)
}

func (d *inviteDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	inv, err := d.client.GetInvite(ctx, id.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading invite", err)
		return
	}
	obj, diags := inviteObject(ctx, inv)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
