package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

const managedAgentsNote = "\n\n~> The Managed Agents API is in beta. This resource needs `api_key` (a regular workspace API key, not an " +
	"Admin key) and `workspace_id` when the key can reach more than one workspace."

// metadataFromMap converts a Terraform map to the create-time metadata map.
func metadataFromMap(ctx context.Context, v types.Map, diags *diag.Diagnostics) map[string]string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	out := map[string]string{}
	diags.Append(v.ElementsAs(ctx, &out, false)...)
	return out
}

// metadataPatch computes the API patch that turns have into want (nil deletes).
func metadataPatch(ctx context.Context, want, have types.Map, diags *diag.Diagnostics) client.MetadataPatch {
	if want.Equal(have) {
		return nil
	}
	w := metadataFromMap(ctx, want, diags)
	h := metadataFromMap(ctx, have, diags)
	patch := client.MetadataPatch{}
	for k, v := range w {
		patch[k] = &v
	}
	for k := range h {
		if _, ok := w[k]; !ok {
			patch[k] = nil
		}
	}
	return patch
}

// metadataToMap maps API metadata back to state, keeping null when the config
// had none and the API returned an empty map.
func metadataToMap(ctx context.Context, api map[string]string, prior types.Map, diags *diag.Diagnostics) types.Map {
	if len(api) == 0 && (prior.IsNull() || prior.IsUnknown()) {
		return types.MapNull(types.StringType)
	}
	if api == nil {
		api = map[string]string{}
	}
	m, d := types.MapValueFrom(ctx, types.StringType, api)
	diags.Append(d...)
	return m
}
