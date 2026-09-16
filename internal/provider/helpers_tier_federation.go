package provider

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var httpsRegexp = regexp.MustCompile(`^https://`)

// httpsURL rejects a URL that is not https. It guards every attribute whose
// value decides where a token, a secret or a signing key is sent or fetched
// from, so a typo or a hostile value cannot move that to a cleartext host.
func httpsURL() validator.String {
	return stringvalidator.RegexMatches(httpsRegexp, "must start with https://")
}

// basetypesObjectAsOptions lets nested objects be decoded into structs whose
// fields may be unhandled null/unknown values.
var basetypesObjectAsOptions = basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}
