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

// Values a group may switch an attribute to.
//
// Skipped unless OMNI_TEST_GROUP_UUID is set, and setting it means reading a
// value out of the database, because no API returns it. The endpoint resolves
// group_id against user_groups.id, a gen_random_uuid() primary key, while
// every other group-scoped endpoint in v1 - model roles, connection roles,
// folder and document permissions - takes the eight-character mini_uuid that
// SCIM calls the group's id. The two columns are generated independently, so
// the mini cannot be converted into the UUID by any client.
//
// The user path is covered separately below, and needs its own fixture for
// the opposite reason: a group's UUID is unobtainable, while a user's
// membership id is simply not something a test can create.
func TestAccUserAttributeSelectableValues_group(t *testing.T) {
	name := accIdentifier("sv")
	groupUUID := requireEnv(t, "OMNI_TEST_GROUP_UUID")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSelectableValuesDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccSelectableValuesConfig(name, groupUUID, `["anz", "apac"]`, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "values.#", "2"),
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "values.0", "anz"),
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "allow_any_value", "false"),
					resource.TestCheckResourceAttr("omni_user_attribute_selectable_values.test", "group_id", groupUUID),
				),
			},
			{
				Config:   testAccSelectableValuesConfig(name, groupUUID, `["anz", "apac"]`, false),
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
				Config: testAccSelectableValuesConfig(name, groupUUID, `["anz", "apac", "emea"]`, true),
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

// A mini_uuid in group_id is rejected at plan time rather than by the API.
//
// This is the mistake the resource invites: every other group-scoped resource
// in the provider takes omni_user_group.id, so reaching for it here is the
// natural thing to do, and the API's answer is a bare "group_id: Invalid UUID"
// partway through an apply. The validator turns that into a plan-time error
// that explains itself.
//
// Vcf6fCLU is the shape SCIM returns, eight characters, not a real group here.
// Nothing is created: the plan fails before any request is made.
func TestAccUserAttributeSelectableValues_groupIDMustBeUUID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccSelectableValuesConfig(accIdentifier("sv"), "Vcf6fCLU", `["anz"]`, false),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`must be a UUID, not the eight-character id`),
			},
		},
	})
}

func testAccSelectableValuesConfig(name, groupUUID, values string, allowAny bool) string {
	// No omni_user_group here. A group this created would carry a mini_uuid
	// as its id, which this endpoint rejects, so the subject has to be a UUID
	// supplied from outside.
	return fmt.Sprintf(`
resource "omni_user_attribute" "test" {
  name  = %[1]q
  label = "Acc Selectable"
  type  = "String"
}

resource "omni_user_attribute_selectable_values" "test" {
  user_attribute_id = omni_user_attribute.test.id
  group_id          = %[2]q
  values            = %[3]s
  allow_any_value   = %[4]t
}
`, name, groupUUID, values, allowAny)
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
