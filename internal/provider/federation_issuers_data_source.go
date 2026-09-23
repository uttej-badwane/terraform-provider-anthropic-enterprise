package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewFederationIssuersDataSource) }

var _ datasource.DataSourceWithConfigure = &federationIssuersDataSource{}

// NewFederationIssuersDataSource returns the anthropic_federation_issuers data source.
func NewFederationIssuersDataSource() datasource.DataSource { return &federationIssuersDataSource{} }

type federationIssuersDataSource struct{ client *client.Client }

type federationIssuersModel struct {
	IncludeArchived types.Bool `tfsdk:"include_archived"`
	Issuers         types.List `tfsdk:"issuers"`
}

var attrTypesIssuerSummary = map[string]attr.Type{
	"id":                       types.StringType,
	"name":                     types.StringType,
	"issuer_url":               types.StringType,
	"check_jti":                types.BoolType,
	"max_jwt_lifetime_seconds": types.Int64Type,
	"jwks_type":                types.StringType,
	"created_at":               types.StringType,
	"archived_at":              types.StringType,
	"created_by_actor_id":      types.StringType,
	"updated_by_actor_id":      types.StringType,
	"archived_by_actor_id":     types.StringType,
}

func (d *federationIssuersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_issuers"
}

func (d *federationIssuersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists workload identity federation issuers. Requires `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"include_archived": schema.BoolAttribute{MarkdownDescription: "Include archived issuers. Defaults to `false`.", Optional: true},
			"issuers": schema.ListNestedAttribute{
				MarkdownDescription: "Issuers.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":                       dsString("Issuer id."),
					"name":                     dsString("Slug name."),
					"issuer_url":               dsString("Expected `iss` claim."),
					"check_jti":                dsBool("Whether `jti` replay checking is enabled."),
					"max_jwt_lifetime_seconds": dsInt64("Maximum accepted token lifetime."),
					"jwks_type":                dsString("Key source type (`discovery`, `explicit_url`, `inline`)."),
					"created_at":               dsString("Creation timestamp."),
					"archived_at":              dsString("Archive timestamp; null while live."),
					"created_by_actor_id":      dsString("Id of the user or service account that created it."),
					"updated_by_actor_id":      dsString("Id of the user or service account that last updated it."),
					"archived_by_actor_id":     dsString("Id of the user or service account that archived it; null while live."),
				}},
			},
		},
	}
}

func (d *federationIssuersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredOAuth, &resp.Diagnostics)
}

func (d *federationIssuersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg federationIssuersModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListFederationIssuers(ctx, cfg.IncludeArchived.ValueBool())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing federation issuers", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for _, is := range list {
		obj, diags := types.ObjectValue(attrTypesIssuerSummary, map[string]attr.Value{
			"id":                       types.StringValue(is.ID),
			"name":                     types.StringValue(is.Name),
			"issuer_url":               types.StringValue(is.IssuerURL),
			"check_jti":                types.BoolValue(is.CheckJTI),
			"max_jwt_lifetime_seconds": types.Int64Value(is.MaxJWTLifetimeSeconds),
			"jwks_type":                types.StringValue(is.JWKS.Type),
			"created_at":               types.StringValue(is.CreatedAt),
			"archived_at":              stringFromPtr(is.ArchivedAt),
			"created_by_actor_id":      stringFromPtr(is.CreatedByActorID),
			"updated_by_actor_id":      stringFromPtr(is.UpdatedByActorID),
			"archived_by_actor_id":     stringFromPtr(is.ArchivedByActorID),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesIssuerSummary}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.Issuers = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
