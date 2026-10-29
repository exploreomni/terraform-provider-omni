package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/exploreomni/terraform-provider-omni/internal/client"
)

// What resourcevalidator.ExactlyOneOf reports when user_id and group_id are
// both set or both absent.
//
// Matched on the detail rather than the summary: the two cases get different
// summaries - "Missing Attribute Configuration" for neither and "Invalid
// Attribute Combination" for both - and the summaries have changed between
// framework versions. This sentence has not.
var regexpInvalidCombination = regexp.MustCompile(`Exactly one of these attributes must be configured`)

// Values a group may switch an attribute to, for a group the same
// configuration creates. This is the case the endpoint could not support while
// it wanted the group's UUID.
func TestAccUserAttributeSelectableValues_group(t *testing.T) {
	name := accIdentifier("sv")

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
					resource.TestCheckResourceAttrPair("omni_user_attribute_selectable_values.test", "group_id", "omni_user_group.test", "id"),
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

// The user path, with the membership id supplied from the environment.
//
// OMNI_TEST_MEMBER_USER_ID is the same value the group membership case uses: a
// user's id as SCIM reports it, which is the membership id this endpoint wants.
// The test cannot make its own subject - omni_user is a real seat, and it
// cannot adopt an account that already exists.
func TestAccUserAttributeSelectableValues_user(t *testing.T) {
	name := accIdentifier("sv-user")
	membershipID := requireEnv(t, "OMNI_TEST_MEMBER_USER_ID")

	config := fmt.Sprintf(`
resource "omni_user_attribute" "test" {
  name  = %[1]q
  label = "Acc Selectable User"
  type  = "String"
}

resource "omni_user_attribute_selectable_values" "test" {
  user_attribute_id = omni_user_attribute.test.id
  user_id           = %[2]q
  values            = ["anz"]
}
`, name, membershipID)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSelectableValuesDestroyed,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "values.#", "1"),
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "user_id", membershipID),
					// The composite id carries the subject kind, so this also
					// asserts a user subject is not recorded as a group one.
					resource.TestCheckResourceAttrWith("omni_user_attribute_selectable_values.test", "id", func(value string) error {
						if !strings.Contains(value, ":user:") {
							return fmt.Errorf("id %q does not identify a user subject", value)
						}
						return nil
					}),
				),
			},
			{
				// The read-back is the real assertion. A wrong id provisions
				// nothing and still answers 200, so only a plan that comes back
				// empty proves the write landed on this subject.
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// A UUID in group_id is rejected at plan time. The endpoint used to want one
// and now answers it with 404 User group does not exist partway through an
// apply, so anyone upgrading with a hard-coded UUID gets a plan error that
// names the value to use instead. Nothing is created.
func TestAccUserAttributeSelectableValues_groupUUIDRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "omni_user_attribute" "test" {
  name  = %q
  label = "Acc Selectable"
  type  = "String"
}

resource "omni_user_attribute_selectable_values" "test" {
  user_attribute_id = omni_user_attribute.test.id
  group_id          = "5a7b9d67-cd43-4724-a9cd-30bf1bbbd847"
  values            = ["anz"]
}
`, accIdentifier("sv")),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Group UUID no longer accepted`),
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
