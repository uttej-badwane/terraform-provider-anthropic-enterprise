package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewAnalyticsArtifactsDataSource) }

var _ datasource.DataSourceWithConfigure = &analyticsArtifactsDataSource{}

// NewAnalyticsArtifactsDataSource returns the anthropic_analytics_artifacts data source.
func NewAnalyticsArtifactsDataSource() datasource.DataSource { return &analyticsArtifactsDataSource{} }

type analyticsArtifactsDataSource struct{ client *client.Client }

type analyticsArtifactsModel struct {
	Date      types.String `tfsdk:"date"`
	Filters   types.List   `tfsdk:"filters"`
	GroupBy   types.List   `tfsdk:"group_by"`
	LimitRows types.Int64  `tfsdk:"limit_rows"`
	Rows      types.List   `tfsdk:"rows"`
}

func artifactRowAttrs() map[string]schema.Attribute {
	return groupDimAttrs(map[string]schema.Attribute{
		"artifact_type":                     dsString("Canonical artifact MIME type, or `other`."),
		"is_shared":                         dsBool("Whether the artifacts in this bucket have ever been shared."),
		"artifacts_created_count":           dsInt64("Artifacts created."),
		"published_artifacts_created_count": dsInt64("Artifacts published to anyone with the link."),
		"distinct_user_count":               dsInt64("Distinct creators."),
	})
}

func (d *analyticsArtifactsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_analytics_artifacts"
}

func (d *analyticsArtifactsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Artifact-creation activity for one day, broken out by MIME type and share state, for a Claude Enterprise " +
			"organization." + analyticsNote,
		Attributes: map[string]schema.Attribute{
			"date":       dateAttr("UTC day, `YYYY-MM-DD`.", true),
			"filters":    filtersAttr([]string{"artifact_type", "is_shared", "product", "rbac_group_id", "user_id"}),
			"group_by":   groupByAttr([]string{"product", "rbac_group_id", "user_id"}),
			"limit_rows": limitRowsAttr,
			"rows": schema.ListNestedAttribute{MarkdownDescription: "One row per (artifact_type, is_shared) bucket, times group dimensions.", Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: artifactRowAttrs()}},
		},
	}
}

func (d *analyticsArtifactsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAnalytics, &resp.Diagnostics)
}

func (d *analyticsArtifactsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg analyticsArtifactsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := client.AnalyticsListParams{Date: cfg.Date.ValueString(), Filters: listStrings(&resp.Diagnostics, cfg.Filters), GroupBy: listStrings(&resp.Diagnostics, cfg.GroupBy)}
	rows, err := d.client.ListAnalyticsArtifacts(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading analytics artifacts", err)
		return
	}
	rows = capRows(rows, cfg.LimitRows)
	vals := make([]map[string]attr.Value, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		vals = append(vals, groupDimValues(map[string]attr.Value{
			"artifact_type":                     types.StringValue(r.ArtifactType),
			"is_shared":                         types.BoolValue(r.IsShared),
			"artifacts_created_count":           types.Int64Value(r.ArtifactsCreatedCount),
			"published_artifacts_created_count": types.Int64Value(r.PublishedArtifactsCreatedCount),
			"distinct_user_count":               types.Int64Value(r.DistinctUserCount),
		}, r.Product, r.RBACGroupID, r.RBACGroupName, r.UserID))
	}
	cfg.Rows = objectList(&resp.Diagnostics, attrTypesFromSchema(artifactRowAttrs()), vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
