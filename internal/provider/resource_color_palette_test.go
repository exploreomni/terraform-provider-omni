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

// Covers a list attribute, which is where ordering and element-count drift
// usually show up: the API may return colors in a different order than they
// were sent, and a plan after apply would never be empty again.
func TestAccColorPalette_basic(t *testing.T) {
	name := accName("palette")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckColorPaletteDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccColorPaletteConfig(name, []string{"#1f77b4", "#ff7f0e", "#2ca02c"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_color_palette.test", "name", name),
					resource.TestCheckResourceAttr("omni_color_palette.test", "type", "discrete"),
					resource.TestCheckResourceAttr("omni_color_palette.test", "colors.#", "3"),
					// Index 0 specifically: a list is ordered, and a palette
					// returned in another order is drift, not a detail.
					resource.TestCheckResourceAttr("omni_color_palette.test", "colors.0", "#1f77b4"),
					resource.TestCheckResourceAttr("omni_color_palette.test", "colors.2", "#2ca02c"),
					resource.TestCheckResourceAttrSet("omni_color_palette.test", "id"),
					resource.TestCheckResourceAttrSet("omni_color_palette.test", "created_at"),
				),
			},
			{
				Config:   testAccColorPaletteConfig(name, []string{"#1f77b4", "#ff7f0e", "#2ca02c"}),
				PlanOnly: true,
			},
			{
				ResourceName:      "omni_color_palette.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Both a changed element and a changed length.
				Config: testAccColorPaletteConfig(name, []string{"#d62728", "#9467bd"}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("omni_color_palette.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_color_palette.test", "colors.#", "2"),
					resource.TestCheckResourceAttr("omni_color_palette.test", "colors.0", "#d62728"),
				),
			},
		},
	})
}

func testAccColorPaletteConfig(name string, colors []string) string {
	quoted := ""
	for i, c := range colors {
		if i > 0 {
			quoted += ", "
		}
		quoted += fmt.Sprintf("%q", c)
	}
	return fmt.Sprintf(`
resource "omni_color_palette" "test" {
  name   = %[1]q
  type   = "discrete"
  colors = [%[2]s]
}
`, name, quoted)
}

func testAccCheckColorPaletteDestroyed(state *terraform.State) error {
	c, err := sweepClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	for _, rs := range resourcesOfType(state, "omni_color_palette") {
		_, err := c.GetColorPalette(ctx, rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("color palette %q still exists after destroy", rs.Primary.ID)
		}
		if !client.IsNotFound(err) {
			return fmt.Errorf("checking color palette %q was destroyed: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
