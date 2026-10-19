package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Needs a model and its connection, which these tests cannot create cheaply: a
// model needs a connection, and a connection needs a warehouse and
// credentials. Both IDs come from the environment, and the case skips without
// them rather than failing a run that is otherwise fine.
//
// Two things are being proved. A role change is an update, not a replacement,
// because a replacement leaves a window with no grant at all. And a destroy is
// not a destroy: the API has no endpoint for removing an assignment, so the
// provider writes role_on_destroy instead and Terraform reports a destroy for
// something that is still there in another form. That is the one place in this
// provider where the verb lies, so it gets an assertion rather than a note.
func TestAccUserGroupModelRole_updateAndDowngradeOnDestroy(t *testing.T) {
	name := accName("role-group")
	modelID := requireEnv(t, "OMNI_TEST_MODEL_ID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGroupModelRoleDowngraded(modelID),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupModelRoleConfig(name, modelID, "QUERIER"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_group_model_role.test", "role_name", "QUERIER"),
					resource.TestCheckResourceAttr("omni_user_group_model_role.test", "model_id", modelID),
					// The documented default, and what makes the destroy
					// assertion below meaningful.
					resource.TestCheckResourceAttr("omni_user_group_model_role.test", "role_on_destroy", "NO_ACCESS"),
				),
			},
			{
				Config:   testAccUserGroupModelRoleConfig(name, modelID, "QUERIER"),
				PlanOnly: true,
			},
			{
				ResourceName:      "omni_user_group_model_role.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccUserGroupModelRoleConfig(name, modelID, "VIEWER"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						// In place. A replacement would revoke before
						// granting, which on a live tenant is an outage.
						plancheck.ExpectResourceAction("omni_user_group_model_role.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr(
					"omni_user_group_model_role.test", "role_name", "VIEWER"),
			},
		},
	})
}

func testAccUserGroupModelRoleConfig(name, modelID, roleName string) string {
	return fmt.Sprintf(`
resource "omni_user_group" "test" {
  display_name = %[1]q
}

resource "omni_user_group_model_role" "test" {
  user_group_id = omni_user_group.test.id
  model_id      = %[2]q
  role_name     = %[3]q
}
`, name, modelID, roleName)
}

// The assignment is expected to survive as NO_ACCESS, not to disappear.
//
// The group is destroyed in the same run, so its roles usually go with it and
// the read returns nothing. What this must never find is the role still
// sitting at QUERIER or VIEWER, which would mean the downgrade silently did
// not happen and access outlived the destroy Terraform reported.
func testAccCheckGroupModelRoleDowngraded(modelID string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		c, err := sweepClient()
		if err != nil {
			return err
		}
		ctx := context.Background()

		for _, rs := range resourcesOfType(state, "omni_user_group_model_role") {
			groupID := rs.Primary.Attributes["user_group_id"]

			roles, err := c.ListGroupModelRoles(ctx, groupID)
			if err != nil {
				// The group went with the destroy, so there is nothing left
				// that could hold a role.
				return nil
			}

			for _, role := range roles {
				if role.ModelID != modelID {
					continue
				}
				// Omni reports every role that applies, including a
				// connection's base role, so only an explicit grant on this
				// model counts.
				if role.RoleName != "" && role.RoleName != "NO_ACCESS" {
					return fmt.Errorf(
						"group %q still holds %q on model %q after destroy; the downgrade to role_on_destroy did not take effect",
						groupID, role.RoleName, modelID,
					)
				}
			}
		}
		return nil
	}
}
