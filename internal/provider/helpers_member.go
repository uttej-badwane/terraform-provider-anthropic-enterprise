package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// writableOrgRoles are the organization roles the API accepts on invite
// create and user update.
var writableOrgRoles = []string{"user", "developer", "billing", "claude_code_user", "managed"}

// memberClientFromResource is for users and invites, which exist on both the
// Console and Claude Enterprise surfaces: any admin or enterprise credential works.
func memberClientFromResource(req resource.ConfigureRequest, diags *diag.Diagnostics) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", "expected *client.Client")
		return nil
	}
	if !c.HasAdmin() && !c.HasEnterprise() {
		diags.AddError("Missing credential for this resource", (&client.MissingCredentialError{Class: client.CredAdmin}).Error())
		return nil
	}
	return c
}

func memberClientFromDataSource(req datasource.ConfigureRequest, diags *diag.Diagnostics) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", "expected *client.Client")
		return nil
	}
	if !c.HasAdmin() && !c.HasEnterprise() {
		diags.AddError("Missing credential for this data source", (&client.MissingCredentialError{Class: client.CredAdmin}).Error())
		return nil
	}
	return c
}
