package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/exploreomni/terraform-provider-omni/internal/client"
)

// Create, read with no drift, import, and update in place.
//
// name is Optional and Computed, so the first step sets it and the state
// should hold exactly that. label and description are what an update changes;
// neither may turn into a replacement, because a replacement drops every value
// users hold for the attribute.
func TestAccUserAttribute_basic(t *testing.T) {
	name := accIdentifier("attr")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserAttributeDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccUserAttributeConfig(name, "Acc Test", "first description", "String", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_attribute.test", "name", name),
					resource.TestCheckResourceAttr("omni_user_attribute.test", "label", "Acc Test"),
					resource.TestCheckResourceAttr("omni_user_attribute.test", "description", "first description"),
					resource.TestCheckResourceAttr("omni_user_attribute.test", "type", "String"),
					resource.TestCheckResourceAttr("omni_user_attribute.test", "multiple_values", "false"),
					// Not a built-in. If this ever reads true, the provider has
					// adopted one of Omni's own attributes.
					resource.TestCheckResourceAttr("omni_user_attribute.test", "system", "false"),
					resource.TestCheckResourceAttrSet("omni_user_attribute.test", "id"),
				),
			},
			{
				Config:   testAccUserAttributeConfig(name, "Acc Test", "first description", "String", false),
				PlanOnly: true,
			},
			{
				ResourceName:      "omni_user_attribute.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccUserAttributeConfig(name, "Acc Test Renamed", "second description", "String", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("omni_user_attribute.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_user_attribute.test", "label", "Acc Test Renamed"),
					resource.TestCheckResourceAttr("omni_user_attribute.test", "description", "second description"),
				),
			},
		},
	})
}

// multiple_values widens in place and narrows by replacement.
//
// This is the behaviour nothing has ever proved. Widening is safe: an
// attribute holding one value can hold several. Narrowing is not, because the
// API will not do it, so the provider's plan modifier turns it into a
// replacement - and a replacement drops every value users hold.
//
// The modifier also emits a warning saying so. The test framework does not
// surface warnings, so this asserts the action and not the text; read the
// warning by hand once while running this.
func TestAccUserAttribute_multipleValuesNarrowingReplaces(t *testing.T) {
	name := accIdentifier("attr-multi")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserAttributeDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccUserAttributeConfig(name, "Acc Multi", "", "String", false),
				Check: resource.TestCheckResourceAttr(
					"omni_user_attribute.test", "multiple_values", "false"),
			},
			{
				// Widen. In place.
				Config: testAccUserAttributeConfig(name, "Acc Multi", "", "String", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("omni_user_attribute.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr(
					"omni_user_attribute.test", "multiple_values", "true"),
			},
			{
				// Narrow. Replacement.
				Config: testAccUserAttributeConfig(name, "Acc Multi", "", "String", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("omni_user_attribute.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr(
					"omni_user_attribute.test", "multiple_values", "false"),
			},
		},
	})
}

// type is immutable, so changing it is a replacement rather than an API error
// mid-apply.
func TestAccUserAttribute_typeReplaces(t *testing.T) {
	name := accIdentifier("attr-type")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserAttributeDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccUserAttributeConfig(name, "Acc Type", "", "String", false),
			},
			{
				Config: testAccUserAttributeConfig(name, "Acc Type", "", "Number", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("omni_user_attribute.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("omni_user_attribute.test", "type", "Number"),
			},
		},
	})
}

// A Number attribute stores its default as a string, which is how Omni keeps
// precision past 2^53 - 1. A float64 round trip would turn the value below
// into ...992, so this is the regression test for finding 37.
func TestAccUserAttribute_numberDefaultKeepsPrecision(t *testing.T) {
	name := accIdentifier("attr-bignum")
	const big = "9007199254740993"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserAttributeDestroyed,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "omni_user_attribute" "test" {
  name          = %[1]q
  label         = "Acc Big Number"
  type          = "Number"
  default_value = %[2]q
}
`, name, big),
				Check: resource.TestCheckResourceAttr("omni_user_attribute.test", "default_value", big),
			},
			{
				// The read back is where precision is lost, so plan again.
				Config: fmt.Sprintf(`
resource "omni_user_attribute" "test" {
  name          = %[1]q
  label         = "Acc Big Number"
  type          = "Number"
  default_value = %[2]q
}
`, name, big),
				PlanOnly: true,
			},
		},
	})
}

// The reference name is rejected at plan time rather than by the API.
//
// Both cases here were found by an acceptance run: the harness generated
// "tfacc-...-attr" and the apply failed on a 400 whose message came from
// Omni. The rule was in the schema description the whole time and nothing
// enforced it.
func TestAccUserAttribute_nameValidation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// A hyphen is the usual mistake: it is legal in the label and
				// in every other name in this provider.
				Config: `
resource "omni_user_attribute" "test" {
  name  = "has-a-hyphen"
  label = "Has a hyphen"
  type  = "String"
}
`,
				ExpectError: regexp.MustCompile(`Invalid user attribute name`),
			},
			{
				Config: `
resource "omni_user_attribute" "test" {
  name  = "omni_reserved"
  label = "Reserved"
  type  = "String"
}
`,
				ExpectError: regexp.MustCompile(`Reserved user attribute name`),
			},
		},
	})
}

func testAccUserAttributeConfig(name, label, description, attrType string, multiple bool) string {
	descriptionLine := ""
	if description != "" {
		descriptionLine = fmt.Sprintf("  description     = %q\n", description)
	}
	return fmt.Sprintf(`
resource "omni_user_attribute" "test" {
  name            = %[1]q
  label           = %[2]q
  type            = %[3]q
  multiple_values = %[4]t
%[5]s}
`, name, label, attrType, multiple, descriptionLine)
}

func testAccCheckUserAttributeDestroyed(state *terraform.State) error {
	c, err := sweepClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	for _, rs := range resourcesOfType(state, "omni_user_attribute") {
		_, err := c.FindUserAttributeByID(ctx, rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("user attribute %q still exists after destroy", rs.Primary.ID)
		}
		if !client.IsNotFound(err) {
			return fmt.Errorf("checking user attribute %q was destroyed: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
