package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// objectList builds a list of objects from attribute maps, collecting diagnostics.
func objectList(diags *diag.Diagnostics, attrTypes map[string]attr.Type, rows []map[string]attr.Value) types.List {
	objs := make([]attr.Value, 0, len(rows))
	for _, row := range rows {
		obj, d := types.ObjectValue(attrTypes, row)
		diags.Append(d...)
		objs = append(objs, obj)
	}
	l, d := types.ListValue(types.ObjectType{AttrTypes: attrTypes}, objs)
	diags.Append(d...)
	return l
}

// listStrings reads an optional list attribute into a Go slice (nil when null).
func listStrings(diags *diag.Diagnostics, v types.List) []string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(v.ElementsAs(nil, &out, false)...) //nolint:staticcheck // ctx unused by ElementsAs for primitives
	return out
}

const reportNote = "\n\n~> This is a point-in-time report that is re-read on every plan. Use it for outputs and policy " +
	"checks, not to manage infrastructure."
