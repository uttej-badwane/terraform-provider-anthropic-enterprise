package provider

import (
	"context"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

const analyticsNote = " Data is available from 2026-01-01 with about a one-day lag; the API allows 60 requests per minute " +
	"per organization. Requires `analytics_api_key` (or an `enterprise_api_key` carrying `read:analytics`)." + reportNote

// analyticsListModel is the shared model of the per-entity analytics lists
// (users, skills, connectors, plugins).
type analyticsListModel struct {
	Date         types.String `tfsdk:"date"`
	StartingDate types.String `tfsdk:"starting_date"`
	EndingDate   types.String `tfsdk:"ending_date"`
	Filters      types.List   `tfsdk:"filters"`
	GroupBy      types.List   `tfsdk:"group_by"`
	Order        types.String `tfsdk:"order"`
	OrderBy      types.String `tfsdk:"order_by"`
	LimitRows    types.Int64  `tfsdk:"limit_rows"`
	Rows         types.List   `tfsdk:"rows"`
}

func (m analyticsListModel) params(diags *diag.Diagnostics) client.AnalyticsListParams {
	return client.AnalyticsListParams{
		Date:         m.Date.ValueString(),
		StartingDate: m.StartingDate.ValueString(),
		EndingDate:   m.EndingDate.ValueString(),
		Filters:      listStrings(diags, m.Filters),
		GroupBy:      listStrings(diags, m.GroupBy),
		Order:        m.Order.ValueString(),
		OrderBy:      m.OrderBy.ValueString(),
	}
}

// analyticsListValidators enforces date XOR starting_date and ending_date needing starting_date.
func analyticsListValidators() []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("date"), path.MatchRoot("starting_date")),
	}
}

func dateAttr(desc string, required bool) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: desc,
		Required:            required,
		Optional:            !required,
		Validators:          []validator.String{stringvalidator.RegexMatches(dateRegexp, "must be YYYY-MM-DD")},
	}
}

// filtersAttr is the `dimension:value` filter list restricted to dims.
func filtersAttr(dims []string) schema.ListAttribute {
	re := regexp.MustCompile(`^(` + strings.Join(dims, "|") + `):.+`)
	return optionalStringList("Filters as `dimension:value`; repeat a dimension for OR, mix dimensions for AND. Dimensions: `"+
		strings.Join(dims, "`, `")+"`.",
		listvalidator.ValueStringsAre(stringvalidator.RegexMatches(re, "must be one of the supported dimensions followed by :value")),
		listvalidator.SizeAtMost(100))
}

func groupByAttr(groups []string) schema.ListAttribute {
	return optionalStringList("Dimensions to break rows out by: `"+strings.Join(groups, "`, `")+"`.",
		listvalidator.ValueStringsAre(stringvalidator.OneOf(groups...)))
}

var orderAttr = schema.StringAttribute{
	MarkdownDescription: "Sort direction, `asc` or `desc`.",
	Optional:            true,
	Validators:          []validator.String{stringvalidator.OneOf("asc", "desc")},
}

var limitRowsAttr = schema.Int64Attribute{
	MarkdownDescription: "Maximum number of rows to keep after reading every page. Unlimited when omitted.",
	Optional:            true,
	Validators:          []validator.Int64{int64validator.AtLeast(1)},
}

// analyticsListInputs are the shared input attributes of the per-entity lists.
func analyticsListInputs(dims, groups []string) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"date":          dateAttr("Single UTC day, `YYYY-MM-DD`. Exactly one of `date` or `starting_date` must be set.", false),
		"starting_date": dateAttr("First day of a range, `YYYY-MM-DD` (rollup mode: one row per entity over the range).", false),
		"ending_date": schema.StringAttribute{
			MarkdownDescription: "Exclusive last day of the range, `YYYY-MM-DD`. Only valid with `starting_date`; defaults to today.",
			Optional:            true,
			Validators: []validator.String{stringvalidator.RegexMatches(dateRegexp, "must be YYYY-MM-DD"),
				stringvalidator.AlsoRequires(path.MatchRoot("starting_date"))},
		},
		"filters":    filtersAttr(dims),
		"group_by":   groupByAttr(groups),
		"order":      orderAttr,
		"order_by":   schema.StringAttribute{MarkdownDescription: "Sort field: the endpoint's name column or one of its rankable metrics.", Optional: true},
		"limit_rows": limitRowsAttr,
	}
}

// capRows applies limit_rows.
func capRows[T any](rows []T, limit types.Int64) []T {
	if limit.IsNull() || limit.IsUnknown() || int64(len(rows)) <= limit.ValueInt64() {
		return rows
	}
	return rows[:limit.ValueInt64()]
}

// group dimension attributes shared by skills, connectors, plugins, artifacts.
func groupDimAttrs(m map[string]schema.Attribute) map[string]schema.Attribute {
	m["product"] = dsString("Product surface; present only when grouped by `product`.")
	m["rbac_group_id"] = dsString("RBAC group id; present only when grouped by `rbac_group_id`.")
	m["rbac_group_name"] = dsString("RBAC group name; null when the group was deleted or unresolved.")
	m["user_id"] = dsString("User id; present only when grouped by `user_id`.")
	return m
}

func groupDimValues(m map[string]attr.Value, product, gid, gname, uid *string) map[string]attr.Value {
	m["product"] = stringFromPtr(product)
	m["rbac_group_id"] = stringFromPtr(gid)
	m["rbac_group_name"] = stringFromPtr(gname)
	m["user_id"] = stringFromPtr(uid)
	return m
}

// attrTypesFromSchema derives object attribute types from a flat map of
// primitive computed attributes (string, int64, bool, float64, list of string).
func attrTypesFromSchema(attrs map[string]schema.Attribute) map[string]attr.Type {
	out := make(map[string]attr.Type, len(attrs))
	for k, a := range attrs {
		out[k] = a.GetType()
	}
	return out
}

func readAnalytics(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse, cfg *analyticsListModel) (client.AnalyticsListParams, bool) {
	resp.Diagnostics.Append(req.Config.Get(ctx, cfg)...)
	if resp.Diagnostics.HasError() {
		return client.AnalyticsListParams{}, false
	}
	p := cfg.params(&resp.Diagnostics)
	return p, !resp.Diagnostics.HasError()
}
