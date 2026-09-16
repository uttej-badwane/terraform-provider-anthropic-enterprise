package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

// pathRootID is the path of the conventional root `id` attribute.
var pathRootID = path.Root("id")

// diagList is a local alias so flatten helpers can return diagnostics by value.
type diagList = diag.Diagnostics
