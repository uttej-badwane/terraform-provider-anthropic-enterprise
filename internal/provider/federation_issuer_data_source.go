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

func init() { registerDataSource(NewFederationIssuerDataSource) }

var (
	_ datasource.DataSourceWithConfigure        = &federationIssuerDataSource{}
	_ datasource.DataSourceWithConfigValidators = &federationIssuerDataSource{}
)

// NewFederationIssuerDataSource returns the anthropic_federation_issuer data source.
func NewFederationIssuerDataSource() datasource.DataSource { return &federationIssuerDataSource{} }

type federationIssuerDataSource struct{ client *client.Client }

func (d *federationIssuerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_issuer"
}

func (d *federationIssuerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up one workload identity federation issuer by id or name. Requires `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"id":                       schema.StringAttribute{MarkdownDescription: "Issuer id (`fdis_...`). Exactly one of `id` or `name` must be set.", Optional: true, Computed: true},
			"name":                     schema.StringAttribute{MarkdownDescription: "Slug name; only live (unarchived) issuers are matched.", Optional: true, Computed: true},
			"issuer_url":               dsString("Expected `iss` claim."),
			"check_jti":                dsBool("Whether `jti` replay checking is enabled."),
			"max_jwt_lifetime_seconds": dsInt64("Maximum accepted token lifetime."),
			"jwks_type":                dsString("Key source type (`discovery`, `explicit_url`, `inline`)."),
			"created_at":               dsString("Creation timestamp."),
			"archived_at":              dsString("Archive timestamp; null while live."),
		},
	}
}

func (d *federationIssuerDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name"))}
}

func (d *federationIssuerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredOAuth, &resp.Diagnostics)
}

func (d *federationIssuerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var is *client.FederationIssuer
	if !id.IsNull() {
		got, err := d.client.GetFederationIssuer(ctx, id.ValueString())
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error reading federation issuer", err)
			return
		}
		is = got
	} else {
		list, err := d.client.ListFederationIssuers(ctx, false)
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error listing federation issuers", err)
			return
		}
		for i := range list {
			if list[i].Name == name.ValueString() {
				if is != nil {
					resp.Diagnostics.AddError("Ambiguous federation issuer name", fmt.Sprintf("more than one live issuer is named %q; use id", name.ValueString()))
					return
				}
				is = &list[i]
			}
		}
		if is == nil {
			resp.Diagnostics.AddError("Federation issuer not found", fmt.Sprintf("no live federation issuer named %q", name.ValueString()))
			return
		}
	}
	obj, diags := types.ObjectValue(attrTypesIssuerSummary, map[string]attr.Value{
		"id":                       types.StringValue(is.ID),
		"name":                     types.StringValue(is.Name),
		"issuer_url":               types.StringValue(is.IssuerURL),
		"check_jti":                types.BoolValue(is.CheckJTI),
		"max_jwt_lifetime_seconds": types.Int64Value(is.MaxJWTLifetimeSeconds),
		"jwks_type":                types.StringValue(is.JWKS.Type),
		"created_at":               types.StringValue(is.CreatedAt),
		"archived_at":              stringFromPtr(is.ArchivedAt),
	})
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
