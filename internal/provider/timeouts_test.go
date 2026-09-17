package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The resources that wait for the Admin API to converge accept a timeouts
// block bounding that wait. A configuration carrying one must plan and apply
// like any other; the durations only change how long the provider is willing
// to wait, never what it sends.
func TestAccTimeoutsBlockAccepted(t *testing.T) {
	skipUnlessMock(t)
	userID := testMock.UserByEmail("dev@example.com").ID
	config := providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" {
  name = "tf-acc-timeouts"
}

resource "anthropic_workspace_member" "test" {
  workspace_id   = anthropic_workspace.test.id
  user_id        = %q
  workspace_role = "workspace_developer"

  timeouts {
    create = "45s"
    update = "45s"
    delete = "45s"
  }
}
`, userID)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			// A timeouts block must not itself produce a diff on re-plan.
			{Config: config, PlanOnly: true},
		},
	})
}
