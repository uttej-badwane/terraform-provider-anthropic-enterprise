package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRBACRolesDataSources(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
data "anthropic_rbac_roles" "all" {}

data "anthropic_rbac_role" "first" {
  id = data.anthropic_rbac_roles.all.roles[0].id
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_rbac_roles.all", tfjsonpath.New("roles"), knownvalue.ListPartial(map[int]knownvalue.Check{
						0: knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.StringExact("Member")}),
						1: knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.StringExact("Project Editor")}),
					})),
					statecheck.ExpectKnownValue("data.anthropic_rbac_role.first", tfjsonpath.New("name"), knownvalue.StringExact("Member")),
					statecheck.ExpectKnownValue("data.anthropic_rbac_role.first", tfjsonpath.New("permissions"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{"action": knownvalue.StringExact("chat"), "resource_type": knownvalue.StringExact("organization"), "resource_id": knownvalue.NotNull()}),
						knownvalue.ObjectPartial(map[string]knownvalue.Check{"action": knownvalue.StringExact("use"), "resource_type": knownvalue.StringExact("all_connectors"), "resource_id": knownvalue.Null()}),
					})),
				},
			},
		},
	})
}
