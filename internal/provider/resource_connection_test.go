package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// omni_connection had no acceptance test, on the assumption that creating a
// connection needs real warehouse credentials. It does not: Omni stores a
// connection without testing it. This connection is never reached, because
// nothing here runs a query or a schema refresh against it.

// The environment user attribute, set and then cleared.
//
// Clearing is the step that matters. Removal means sending
// environmentUserAttribute: null on a PATCH, and an omitted field leaves the
// current value in place, so a provider that dropped the field instead of
// sending null would report success while changing nothing. Terraform state
// cannot show that: it records what the configuration asked for. So each step
// reads the connection back from the API and checks what Omni actually holds.
func TestAccConnection_environmentUserAttribute(t *testing.T) {
	connection := accName("conn-env")
	attribute := accIdentifier("connenv")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectionEnvironmentConfig(connection, attribute, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_connection.test", "environment_user_attribute_name", attribute),
					resource.TestCheckResourceAttr("omni_connection.test", "environment_user_attribute_default_values.#", "1"),
					resource.TestCheckResourceAttr("omni_connection.test", "environment_user_attribute_default_values.0", "internal"),
					checkConnectionEnvironmentAttribute(attribute, []string{"internal"}),
				),
			},
			{
				// The read side: the attribute comes back under different, flat
				// field names from the nested object the PATCH sends. A mapping
				// mistake there shows up as a diff on the next plan.
				Config:   testAccConnectionEnvironmentConfig(connection, attribute, true),
				PlanOnly: true,
			},
			{
				Config: testAccConnectionEnvironmentConfig(connection, attribute, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("omni_connection.test", "environment_user_attribute_name"),
					checkConnectionEnvironmentAttribute("", nil),
				),
			},
			{
				Config:   testAccConnectionEnvironmentConfig(connection, attribute, false),
				PlanOnly: true,
			},
		},
	})
}

func testAccConnectionEnvironmentConfig(connection, attribute string, withEnvironment bool) string {
	environment := ""
	if withEnvironment {
		environment = `
  environment_user_attribute_name           = omni_user_attribute.routing.name
  environment_user_attribute_default_values = ["internal"]`
	}

	return fmt.Sprintf(`
resource "omni_user_attribute" "routing" {
  name  = %[2]q
  label = "Acc Routing"
  type  = "String"
}

resource "omni_connection" "test" {
  name      = %[1]q
  dialect   = "snowflake"
  host      = "tfacc-dummy"
  database  = "DUMMY"
  warehouse = "DUMMY_WH"
  username  = "DUMMY"
  password  = "dummy"
%[3]s
}
`, connection, attribute, environment)
}

// stateConnectionID returns the connection's id from state, for checks that
// query the API directly.
func stateConnectionID(state *terraform.State) (string, error) {
	connections := resourcesOfType(state, "omni_connection")
	if len(connections) != 1 {
		return "", fmt.Errorf("expected one omni_connection in state, found %d", len(connections))
	}
	return connections[0].Primary.ID, nil
}

// checkConnectionEnvironmentAttribute asserts what Omni holds, not what state
// says. An empty wantName means the attribute should have been cleared.
func checkConnectionEnvironmentAttribute(wantName string, wantValues []string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		connectionID, err := stateConnectionID(state)
		if err != nil {
			return err
		}
		c, err := sweepClient()
		if err != nil {
			return err
		}
		connection, err := c.GetConnection(context.Background(), connectionID)
		if err != nil {
			return err
		}

		gotName := connection.UserAttributeNameForConnectionEnvironments
		if gotName != wantName {
			if wantName == "" {
				return fmt.Errorf("the environment user attribute is still %q in Omni after it was "+
					"removed from configuration: the PATCH did not clear it", gotName)
			}
			return fmt.Errorf("Omni holds environment user attribute %q, expected %q", gotName, wantName)
		}

		gotValues := connection.UserAttributeValuesForDefaultEnvironment
		if len(gotValues) != len(wantValues) {
			return fmt.Errorf("Omni holds default values %v, expected %v", gotValues, wantValues)
		}
		for i := range wantValues {
			if gotValues[i] != wantValues[i] {
				return fmt.Errorf("Omni holds default values %v, expected %v", gotValues, wantValues)
			}
		}
		return nil
	}
}
