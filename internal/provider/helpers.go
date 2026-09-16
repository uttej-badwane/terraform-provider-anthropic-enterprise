package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// clientFromResource extracts the configured client and verifies that the
// credential class the resource needs is present.
func clientFromResource(req resource.ConfigureRequest, class client.CredentialClass, diags *diag.Diagnostics) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return nil
	}
	if !c.Has(class) {
		diags.AddError("Missing credential for this resource", (&client.MissingCredentialError{Class: class}).Error())
		return nil
	}
	return c
}

func clientFromDataSource(req datasource.ConfigureRequest, class client.CredentialClass, diags *diag.Diagnostics) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return nil
	}
	if !c.Has(class) {
		diags.AddError("Missing credential for this data source", (&client.MissingCredentialError{Class: class}).Error())
		return nil
	}
	return c
}

// compositeID joins parent/child identifiers for resources addressed by two ids.
func compositeID(parts ...string) string { return strings.Join(parts, "/") }

// splitCompositeID splits an import id of the form parent/child into two parts.
func splitCompositeID(id string, format string) ([]string, error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("unexpected import id %q: expected %s", id, format)
	}
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("unexpected import id %q: expected %s", id, format)
		}
	}
	return parts, nil
}

// stringPtr returns nil for null/unknown values, otherwise a pointer to the string.
func stringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// stringFromPtr maps nil to a null string.
func stringFromPtr(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// int64Ptr returns nil for null/unknown values.
func int64Ptr(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := v.ValueInt64()
	return &i
}

func int64FromPtr(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

func boolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// stringList converts a Go slice to a types.List of strings (null when nil).
func stringList(ctx context.Context, in []string) (types.List, diag.Diagnostics) {
	if in == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, in)
}

// apiErrorDiag formats an API failure for a diagnostic.
func apiErrorDiag(diags *diag.Diagnostics, summary string, err error) {
	diags.AddError(summary, err.Error())
}
