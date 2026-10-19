package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/exploreomni/terraform-provider-omni/internal/client"
)

// Requires an organization API key: groups go through SCIM.
//
// The rename assertion is the one that matters. Every module in this
// repository treats the configuration key as the group's identity and
// display_name as a label that can change freely. If a rename were a
// replacement, each label edit would cost the group its membership and its
// grants, and that convention would be wrong everywhere.
func TestAccUserGroup_basic(t *testing.T) {
	name := accName("group")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_group.test", "display_name", name),
					resource.TestCheckResourceAttrSet("omni_user_group.test", "id"),
				),
			},
			{
				Config:   testAccUserGroupConfig(name),
				PlanOnly: true,
			},
			{
				ResourceName:      "omni_user_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccUserGroupConfig(name + "-renamed"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("omni_user_group.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr(
					"omni_user_group.test", "display_name", name+"-renamed"),
			},
		},
	})
}

// member_ids left unset means Terraform does not own the membership, and an
// unrelated update must not clear it.
//
// This is the regression test for finding 36, where a null member_ids became
// `"members": []` on what is a SCIM replace, so changing only the display name
// emptied the group. It needs a real user to be meaningful, so it skips
// without one: an empty group would pass whether or not the bug were back.
func TestAccUserGroup_unmanagedMembershipSurvivesUpdate(t *testing.T) {
	name := accName("group-unmgd")
	memberID := requireEnv(t, "OMNI_TEST_MEMBER_USER_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserGroupDestroyed,
		Steps: []resource.TestStep{
			{
				// Managed first, to put a member in place through Terraform.
				Config: fmt.Sprintf(`
resource "omni_user_group" "test" {
  display_name = %[1]q
  member_ids   = [%[2]q]
}
`, name, memberID),
				Check: resource.TestCheckResourceAttr("omni_user_group.test", "member_ids.#", "1"),
			},
			{
				// Drop member_ids and rename in the same step. Membership is
				// now unmanaged, so the member must survive.
				Config: testAccUserGroupConfig(name + "-renamed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_group.test", "display_name", name+"-renamed"),
					testAccCheckGroupHasMember("omni_user_group.test", memberID),
				),
			},
		},
	})
}

func testAccUserGroupConfig(displayName string) string {
	return fmt.Sprintf(`
resource "omni_user_group" "test" {
  display_name = %[1]q
}
`, displayName)
}

// testAccCheckGroupHasMember asks the API rather than the state. State would
// only say what Terraform believes, and the bug this guards against was the
// API being sent something the state never showed.
func testAccCheckGroupHasMember(resourceName, memberID string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs, ok := state.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("%s not found in state", resourceName)
		}

		c, err := sweepClient()
		if err != nil {
			return err
		}
		group, err := c.GetGroup(context.Background(), rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("reading group %q: %w", rs.Primary.ID, err)
		}

		for _, m := range group.Members {
			if m.Value == memberID {
				return nil
			}
		}
		return fmt.Errorf("group %q no longer has member %q: an update cleared an unmanaged membership", rs.Primary.ID, memberID)
	}
}

func testAccCheckUserGroupDestroyed(state *terraform.State) error {
	c, err := sweepClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	for _, rs := range resourcesOfType(state, "omni_user_group") {
		_, err := c.GetGroup(ctx, rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("group %q still exists after destroy", rs.Primary.ID)
		}
		if !client.IsNotFound(err) {
			return fmt.Errorf("checking group %q was destroyed: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
