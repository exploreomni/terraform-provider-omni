package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

// A label is the simplest resource in the provider and the right place to
// prove the shape every other test follows: create, read back with no drift,
// import, update in place, destroy.
func TestAccLabel_basic(t *testing.T) {
	name := accName("label")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLabelDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccLabelConfig(name, "#336699", "created by an acceptance test"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_label.test", "name", name),
					resource.TestCheckResourceAttr("omni_label.test", "color", "#336699"),
					resource.TestCheckResourceAttr("omni_label.test", "description", "created by an acceptance test"),
					// Defaults the provider applies rather than the API.
					resource.TestCheckResourceAttr("omni_label.test", "homepage", "false"),
					resource.TestCheckResourceAttr("omni_label.test", "verified", "false"),
					// The ID is the name: a label has no identifier of its own.
					resource.TestCheckResourceAttr("omni_label.test", "id", name),
				),
			},
			{
				// No-drift read. An empty plan here is the assertion: if the
				// API returns a field in a different shape than it was sent,
				// this step fails rather than some later apply.
				Config:   testAccLabelConfig(name, "#336699", "created by an acceptance test"),
				PlanOnly: true,
			},
			{
				ResourceName:      "omni_label.test",
				ImportState:       true,
				ImportStateId:     name,
				ImportStateVerify: true,
			},
			{
				Config: testAccLabelConfig(name, "#993366", "edited by an acceptance test"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						// In place, not a replacement. A label carries usage,
						// so a replacement would detach it from everything
						// using it.
						plancheck.ExpectResourceAction("omni_label.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_label.test", "color", "#993366"),
					resource.TestCheckResourceAttr("omni_label.test", "description", "edited by an acceptance test"),
				),
			},
		},
	})
}

func testAccLabelConfig(name, color, description string) string {
	return fmt.Sprintf(`
resource "omni_label" "test" {
  name        = %[1]q
  color       = %[2]q
  description = %[3]q
}
`, name, color, description)
}

func testAccCheckLabelDestroyed(state *terraform.State) error {
	c, err := sweepClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	for _, rs := range resourcesOfType(state, "omni_label") {
		// Read by name, which is what the API keys labels on.
		_, err := c.GetLabel(ctx, rs.Primary.Attributes["name"])
		if err == nil {
			return fmt.Errorf("label %q still exists after destroy", rs.Primary.Attributes["name"])
		}
		if !client.IsNotFound(err) {
			return fmt.Errorf("checking label %q was destroyed: %w", rs.Primary.Attributes["name"], err)
		}
	}
	return nil
}
