package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

// What resourcevalidator.ExactlyOneOf reports when user_id and group_id are
// both set or both absent.
var regexpInvalidCombination = regexp.MustCompile(`Invalid Attribute Combination`)

// Values a group may switch an attribute to. Needs an organization API key,
// because it creates a group.
//
// Group rather than user on purpose. The endpoint takes a *membership* ID for
// a user subject, not a user ID, and it answers 200 either way - so a user case
// built on the wrong ID would pass while provisioning nothing. Until that is
// settled, a user test would assert the wrong thing confidently.
func TestAccUserAttributeSelectableValues_group(t *testing.T) {
	name := accName("sv")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSelectableValuesDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccSelectableValuesConfig(name, `["anz", "apac"]`, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "values.#", "2"),
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "values.0", "anz"),
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "allow_any_value", "false"),
					// Composite, and the subject kind is in it: there is no
					// single API identifier for a provisioning.
					resource.TestCheckResourceAttrWith("omni_user_attribute_selectable_values.test", "id", func(value string) error {
						if len(value) == 0 {
							return fmt.Errorf("empty id")
						}
						return nil
					}),
				),
			},
			{
				Config:   testAccSelectableValuesConfig(name, `["anz", "apac"]`, false),
				PlanOnly: true,
			},
			{
				ResourceName: "omni_user_attribute_selectable_values.test",
				ImportState:  true,
				// The ID is built from two resources, so it has to be read out
				// of state rather than written in the test.
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					rs, ok := state.RootModule().Resources["omni_user_attribute_selectable_values.test"]
					if !ok {
						return "", fmt.Errorf("selectable values not found in state")
					}
					return rs.Primary.ID, nil
				},
				ImportStateVerify: true,
			},
			{
				// A changed set, and the wide grant turned on. Both go through
				// the same PUT, which is why create and update are one call.
				Config: testAccSelectableValuesConfig(name, `["anz", "apac", "emea"]`, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "values.#", "3"),
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "allow_any_value", "true"),
				),
			},
		},
	})
}

// Exactly one subject. The provider rejects both and neither at validate time,
// before any API call, because the API would accept a body with both and do
// something unstated with it.
func TestAccUserAttributeSelectableValues_subjectValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "omni_user_attribute_selectable_values" "test" {
  user_attribute_id = "00000000-0000-0000-0000-000000000000"
  values            = ["a"]
}
`,
				ExpectError: regexpInvalidCombination,
			},
			{
				Config: `
resource "omni_user_attribute_selectable_values" "test" {
  user_attribute_id = "00000000-0000-0000-0000-000000000000"
  user_id           = "11111111-1111-1111-1111-111111111111"
  group_id          = "22222222-2222-2222-2222-222222222222"
  values            = ["a"]
}
`,
				ExpectError: regexpInvalidCombination,
			},
		},
	})
}

func testAccSelectableValuesConfig(name, values string, allowAny bool) string {
	return fmt.Sprintf(`
resource "omni_user_attribute" "test" {
  name  = %[1]q
  label = "Acc Selectable"
  type  = "String"
}

resource "omni_user_group" "test" {
  display_name = %[1]q
}

resource "omni_user_attribute_selectable_values" "test" {
  user_attribute_id = omni_user_attribute.test.id
  group_id          = omni_user_group.test.id
  values            = %[2]s
  allow_any_value   = %[3]t
}
`, name, values, allowAny)
}

// Destroying a provisioning is a write, not a delete: the endpoint has no
// DELETE, so the provider sends an empty list with allow_any_value false.
//
// Both outcomes below are a pass. The attribute is destroyed in the same run
// and the provisioning cannot outlive it, so a not-found is the usual answer;
// an attribute that somehow survived must have nothing provisioned on it.
func testAccCheckSelectableValuesDestroyed(state *terraform.State) error {
	c, err := sweepClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	for _, rs := range resourcesOfType(state, "omni_user_attribute_selectable_values") {
		attrID := rs.Primary.Attributes["user_attribute_id"]
		subject := client.SelectableValuesSubject{
			UserID:  rs.Primary.Attributes["user_id"],
			GroupID: rs.Primary.Attributes["group_id"],
		}

		out, err := c.GetSelectableValues(ctx, attrID, subject)
		if err != nil {
			if client.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("checking selectable values on %q were removed: %w", attrID, err)
		}
		if len(out.Values) > 0 {
			return fmt.Errorf("selectable values on %q still provisioned after destroy: %v", attrID, out.Values)
		}
		if out.AllowAnyValue {
			return fmt.Errorf("allow_any_value still true on %q after destroy, which leaves the grant in place", attrID)
		}
	}
	return nil
}
