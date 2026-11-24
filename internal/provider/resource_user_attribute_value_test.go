package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// These run against a user Terraform does not own, which is the point of the
// resource: OMNI_TEST_MEMBER_USER_ID is an existing person, looked up with the
// data source and never created or destroyed by the test. Each case makes its
// own attribute definition, so the only thing written to that person is a
// tfacc_ key that the destroy clears again.

// A multi-valued attribute, the case omni_user.attributes cannot express.
func TestAccUserAttributeValue_multiple(t *testing.T) {
	userID := requireEnv(t, "OMNI_TEST_MEMBER_USER_ID")
	name := accIdentifier("uav_multi")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserAttributeValueCleared(t, userID, name),
		Steps: []resource.TestStep{
			{
				Config: testAccUserAttributeValueMultipleConfig(userID, name, `["123", "456"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_attribute_value.test", "id", userID+"/"+name),
					resource.TestCheckResourceAttr("omni_user_attribute_value.test", "values.#", "2"),
					resource.TestCheckTypeSetElemAttr("omni_user_attribute_value.test", "values.*", "123"),
					resource.TestCheckTypeSetElemAttr("omni_user_attribute_value.test", "values.*", "456"),
					testAccCheckUserAttributeValueInOmni(t, userID, name, []string{"123", "456"}),
				),
			},
			{
				// Order in config is not order in Omni. A set means no diff.
				Config:   testAccUserAttributeValueMultipleConfig(userID, name, `["456", "123"]`),
				PlanOnly: true,
			},
			{
				Config: testAccUserAttributeValueMultipleConfig(userID, name, `["123", "789"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_attribute_value.test", "values.#", "2"),
					testAccCheckUserAttributeValueInOmni(t, userID, name, []string{"123", "789"}),
				),
			},
			{
				ResourceName:      "omni_user_attribute_value.test",
				ImportState:       true,
				ImportStateId:     userID + "/" + name,
				ImportStateVerify: true,
			},
		},
	})
}

// A single-valued attribute through value.
func TestAccUserAttributeValue_single(t *testing.T) {
	userID := requireEnv(t, "OMNI_TEST_MEMBER_USER_ID")
	name := accIdentifier("uav_single")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserAttributeValueCleared(t, userID, name),
		Steps: []resource.TestStep{
			{
				Config: testAccUserAttributeValueSingleConfig(userID, name, "anz"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_attribute_value.test", "value", "anz"),
					resource.TestCheckNoResourceAttr("omni_user_attribute_value.test", "values.#"),
					testAccCheckUserAttributeValueInOmni(t, userID, name, []string{"anz"}),
				),
			},
			{
				Config:   testAccUserAttributeValueSingleConfig(userID, name, "anz"),
				PlanOnly: true,
			},
			{
				Config: testAccUserAttributeValueSingleConfig(userID, name, "apac"),
				Check:  testAccCheckUserAttributeValueInOmni(t, userID, name, []string{"apac"}),
			},
			{
				ResourceName:      "omni_user_attribute_value.test",
				ImportState:       true,
				ImportStateId:     userID + "/" + name,
				ImportStateVerify: true,
			},
		},
	})
}

// Exactly one of value or values, caught at plan time. Nothing is written.
func TestAccUserAttributeValue_valueOrValues(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "omni_user_attribute_value" "test" {
  user_id = "00000000-0000-0000-0000-000000000000"
  name    = "location_id"
  value   = "123"
  values  = ["123"]
}
`,
				PlanOnly:    true,
				ExpectError: regexpInvalidCombination,
			},
			{
				Config: `
resource "omni_user_attribute_value" "test" {
  user_id = "00000000-0000-0000-0000-000000000000"
  name    = "location_id"
  values  = []
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)values.*at least 1`),
			},
		},
	})
}

func testAccUserAttributeValueMultipleConfig(userID, name, values string) string {
	return fmt.Sprintf(`
data "omni_user" "member" {
  id = %[1]q
}

resource "omni_user_attribute" "test" {
  name            = %[2]q
  label           = "Acc Location"
  type            = "String"
  multiple_values = true
}

resource "omni_user_attribute_value" "test" {
  user_id = data.omni_user.member.id
  name    = omni_user_attribute.test.name
  values  = %[3]s
}
`, userID, name, values)
}

func testAccUserAttributeValueSingleConfig(userID, name, value string) string {
	return fmt.Sprintf(`
data "omni_user" "member" {
  id = %[1]q
}

resource "omni_user_attribute" "test" {
  name  = %[2]q
  label = "Acc Region"
  type  = "String"
}

resource "omni_user_attribute_value" "test" {
  user_id = data.omni_user.member.id
  name    = omni_user_attribute.test.name
  value   = %[3]q
}
`, userID, name, value)
}

// testAccCheckUserAttributeValueInOmni reads the user back over SCIM, so the
// test checks what Omni stored rather than what Terraform remembers sending.
func testAccCheckUserAttributeValueInOmni(t *testing.T, userID, name string, want []string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		attrs, err := testAccClient(t).GetUserAttributeValues(context.Background(), userID)
		if err != nil {
			return err
		}
		raw, ok := attrs[name]
		if !ok {
			return fmt.Errorf("user %s has no %q attribute in Omni", userID, name)
		}
		got, _ := attributeValueStrings(raw)
		sort.Strings(got)
		w := append([]string(nil), want...)
		sort.Strings(w)
		if strings.Join(got, ",") != strings.Join(w, ",") {
			return fmt.Errorf("user %s %q in Omni is %v, want %v", userID, name, got, w)
		}
		return nil
	}
}

// After destroy the key is gone and the person is still there. The second
// half is the promise the resource makes: it never deprovisions anyone.
func testAccCheckUserAttributeValueCleared(t *testing.T, userID, name string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		attrs, err := testAccClient(t).GetUserAttributeValues(context.Background(), userID)
		if err != nil {
			return fmt.Errorf("user %s should still exist after destroy: %w", userID, err)
		}
		if raw, ok := attrs[name]; ok && raw != nil {
			return fmt.Errorf("user %s still has %q = %v after destroy", userID, name, raw)
		}
		return nil
	}
}
