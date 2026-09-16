package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// notPrefixedWith rejects strings that start with prefix (case-insensitive).
func notPrefixedWith(prefix string) validator.String { return prefixValidator{prefix: prefix} }

type prefixValidator struct{ prefix string }

func (v prefixValidator) Description(context.Context) string {
	return fmt.Sprintf("must not start with %q", v.prefix)
}

func (v prefixValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }

func (v prefixValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if strings.HasPrefix(strings.ToLower(req.ConfigValue.ValueString()), strings.ToLower(v.prefix)) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid value",
			fmt.Sprintf("%q must not start with %q", req.ConfigValue.ValueString(), v.prefix))
	}
}

// dsString is a shorthand for a computed string attribute in data sources.
func dsString(desc string) schema.StringAttribute {
	return schema.StringAttribute{MarkdownDescription: desc, Computed: true}
}

// dsStringList is a computed list-of-strings attribute in data sources.
func dsStringList(desc string) schema.ListAttribute {
	return schema.ListAttribute{MarkdownDescription: desc, Computed: true, ElementType: types.StringType}
}

// dsBool is a computed bool attribute in data sources.
func dsBool(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{MarkdownDescription: desc, Computed: true}
}

// dsInt64 is a computed int64 attribute in data sources.
func dsInt64(desc string) schema.Int64Attribute {
	return schema.Int64Attribute{MarkdownDescription: desc, Computed: true}
}
