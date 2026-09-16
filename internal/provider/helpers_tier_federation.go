package provider

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var httpsRegexp = regexp.MustCompile(`^https://`)

// basetypesObjectAsOptions lets nested objects be decoded into structs whose
// fields may be unhandled null/unknown values.
var basetypesObjectAsOptions = basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}
