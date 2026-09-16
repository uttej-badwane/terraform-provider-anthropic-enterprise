package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// An imported workspace must not inherit the destructive default: removing it
// from configuration right after import would otherwise archive a real
// workspace, and every API key scoped to it, irreversibly.
func TestAccWorkspaceResource_importStartsWithArchiveOff(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	config := providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" {
  name = %q
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				ResourceName:      "anthropic_workspace.test",
				ImportState:       true,
				ImportStateVerify: false,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 imported instance, got %d", len(states))
					}
					if got := states[0].Attributes["archive_on_destroy"]; got != "false" {
						return fmt.Errorf("imported archive_on_destroy = %q, want \"false\"", got)
					}
					return nil
				},
			},
		},
	})
}
