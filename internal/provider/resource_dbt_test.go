package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/exploreomni/terraform-provider-omni/internal/client"
)

// dbt attaches to a connection, and the suite does not create connections:
// that needs real warehouse credentials, which is why there is no
// resource_connection_test.go either. So these cases run against a connection
// that already exists on the test instance, named by OMNI_TEST_CONNECTION_ID,
// and skip when it is not set.
//
// The connection's dialect also has to support dbt. GET
// /v1/connections/{id}/dbt reports supportsDbt, and the first case asserts it
// rather than letting a later step fail for a reason that looks unrelated.
const dbtConnectionEnv = "OMNI_TEST_CONNECTION_ID"

// What the resource reports when a variable name appears in both variables and
// secret_variables. Matched on the explanation rather than the summary, which
// framework versions have changed before.
var regexpDuplicateDbtVariable = regexp.MustCompile(`can only appear in one of them`)

// A dbt configuration through its whole lifecycle.
//
// The repository URL does not have to be clonable for any of this: Omni stores
// the configuration on the PUT and only tries to clone during a schema
// refresh. Nothing here triggers one.
func TestAccDbtConfiguration_basic(t *testing.T) {
	connectionID := requireEnv(t, dbtConnectionEnv)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDbtConfigurationDestroyed(connectionID),
		Steps: []resource.TestStep{
			{
				Config: testAccDbtConfigurationConfig(connectionID, "main", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "branch", "main"),
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "auth_method", "ssh"),
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "enable_virtual_schemas", "true"),
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "supports_dbt", "true"),
					// The configuration is a property of the connection, so its
					// id is the connection id rather than one of its own.
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "id", connectionID),
					// ssh auth means Omni generates a deploy key and returns it.
					// A user cannot clone without adding it, so an empty value
					// here would be a silent dead end.
					resource.TestCheckResourceAttrSet("omni_dbt_configuration.test", "public_key"),
				),
			},
			{
				// Read has to be exact. The PUT response omits whatever the auth
				// method does not use, and a field copied blindly over state
				// shows up here as a permanent diff.
				Config:   testAccDbtConfigurationConfig(connectionID, "main", true),
				PlanOnly: true,
			},
			{
				Config: testAccDbtConfigurationConfig(connectionID, "develop", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "branch", "develop"),
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "enable_virtual_schemas", "false"),
				),
			},
			{
				ResourceName:  "omni_dbt_configuration.test",
				ImportState:   true,
				ImportStateId: connectionID,
				// token is write-only and never returned, so an imported
				// configuration cannot carry it. Verifying state would compare
				// against a config value the API will not supply.
				ImportStateVerify: false,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "connection_id", connectionID),
				),
			},
		},
	})
}

// An environment, including the variable path that is easiest to get wrong.
//
// A variable already on the environment has to carry its id on update or the
// API adds a second variable with the same name. The provider does not hold
// those ids in state, it resolves them from the API by name, so the third step
// asserts the count rather than just the value: a duplicate would leave two.
func TestAccDbtEnvironment_variablesUpdateInPlace(t *testing.T) {
	connectionID := requireEnv(t, dbtConnectionEnv)
	name := accIdentifier("dbtenv")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDbtEnvironmentDestroyed(connectionID),
		Steps: []resource.TestStep{
			{
				Config: testAccDbtEnvironmentConfig(connectionID, name, "analytics_staging", `lookback_days = "30"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "name", name),
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "target_schema", "analytics_staging"),
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "variables.lookback_days", "30"),
					resource.TestCheckResourceAttrSet("omni_dbt_environment.test", "id"),
					// Omni decides this; the provider cannot set it.
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "is_default_environment", "false"),
				),
			},
			{
				Config:   testAccDbtEnvironmentConfig(connectionID, name, "analytics_staging", `lookback_days = "30"`),
				PlanOnly: true,
			},
			{
				Config: testAccDbtEnvironmentConfig(connectionID, name, "analytics_staging", `lookback_days = "7"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "variables.lookback_days", "7"),
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "variables.%", "1"),
					checkDbtVariableCount(connectionID, "lookback_days", 1),
				),
			},
		},
	})
}

// A secret variable, which the API never reads back.
//
// The value returns as null, so the provider deliberately does not refresh
// variable values from a response. If it did, the null would overwrite what the
// configuration set and every plan after the first would show a diff. The
// PlanOnly step is the whole point of this case.
func TestAccDbtEnvironment_secretVariableHasNoPerpetualDiff(t *testing.T) {
	connectionID := requireEnv(t, dbtConnectionEnv)
	name := accIdentifier("dbtsec")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDbtEnvironmentDestroyed(connectionID),
		Steps: []resource.TestStep{
			{
				Config: testAccDbtEnvironmentSecretConfig(connectionID, name, "s3cret"),
				Check: resource.TestCheckResourceAttr(
					"omni_dbt_environment.test", "secret_variables.external_api_key", "s3cret"),
			},
			{
				Config:   testAccDbtEnvironmentSecretConfig(connectionID, name, "s3cret"),
				PlanOnly: true,
			},
		},
	})
}

// A name set in both maps is rejected before any request, since Omni stores a
// variable as secret or not and cannot hold it as both.
func TestAccDbtEnvironment_variableCannotBeBothSecretAndNot(t *testing.T) {
	connectionID := requireEnv(t, dbtConnectionEnv)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "omni_dbt_environment" "test" {
  connection_id = %[1]q
  name          = %[2]q
  target_schema = "analytics_clash"

  variables = {
    shared = "plain"
  }

  secret_variables = {
    shared = "secret"
  }
}
`, connectionID, accIdentifier("dbtclash")),
				ExpectError: regexpDuplicateDbtVariable,
			},
		},
	})
}

func testAccDbtConfigurationConfig(connectionID, branch string, virtualSchemas bool) string {
	return fmt.Sprintf(`
resource "omni_dbt_configuration" "test" {
  connection_id = %[1]q

  ssh_url = "git@github.com:exploreomni/terraform-provider-omni-dbt-acc.git"
  branch  = %[2]q

  auth_method            = "ssh"
  enable_virtual_schemas = %[3]t
  autogen_relationships  = false
}
`, connectionID, branch, virtualSchemas)
}

// An environment needs the configuration in place, and enable_virtual_schemas
// is what lets an environment route to another schema at all, so both are
// created here rather than assumed.
func testAccDbtEnvironmentConfig(connectionID, name, schema, variables string) string {
	return fmt.Sprintf(`
%[1]s

resource "omni_dbt_environment" "test" {
  connection_id = omni_dbt_configuration.test.connection_id
  name          = %[2]q
  target_schema = %[3]q

  variables = {
    %[4]s
  }
}
`, testAccDbtConfigurationConfig(connectionID, "main", true), name, schema, variables)
}

func testAccDbtEnvironmentSecretConfig(connectionID, name, secret string) string {
	return fmt.Sprintf(`
%[1]s

resource "omni_dbt_environment" "test" {
  connection_id = omni_dbt_configuration.test.connection_id
  name          = %[2]q
  target_schema = "analytics_secret"

  secret_variables = {
    external_api_key = %[3]q
  }
}
`, testAccDbtConfigurationConfig(connectionID, "main", true), name, secret)
}

// checkDbtVariableCount asserts how many variables of a given name the API
// holds, which is what catches a duplicate the Terraform state would not show:
// state carries one map entry either way.
func checkDbtVariableCount(connectionID, variableName string, want int) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resources := resourcesOfType(state, "omni_dbt_environment")
		if len(resources) != 1 {
			return fmt.Errorf("expected one omni_dbt_environment in state, found %d", len(resources))
		}

		c, err := sweepClient()
		if err != nil {
			return err
		}

		environment, err := c.GetDbtEnvironment(context.Background(), connectionID, resources[0].Primary.ID)
		if err != nil {
			return err
		}

		got := 0
		for _, variable := range environment.Variables {
			if variable.Name == variableName {
				got++
			}
		}
		if got != want {
			return fmt.Errorf("expected %d variables named %q, found %d: an update that omits a "+
				"variable's id adds a duplicate instead of changing it", want, variableName, got)
		}
		return nil
	}
}

// The configuration is removed, not the connection, so a destroyed
// configuration leaves a connection that reports dbt as not configured. That
// is a 200 with a message rather than a 404, which is why this checks
// Configured rather than treating an error as the pass.
func testAccCheckDbtConfigurationDestroyed(connectionID string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		c, err := sweepClient()
		if err != nil {
			return err
		}

		configuration, err := c.GetDbtConfiguration(context.Background(), connectionID)
		if err != nil {
			if client.IsNotFound(err) {
				return nil
			}
			return err
		}
		if configuration.Configured() {
			return fmt.Errorf("dbt is still configured on connection %s after destroy", connectionID)
		}
		return nil
	}
}

func testAccCheckDbtEnvironmentDestroyed(connectionID string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		c, err := sweepClient()
		if err != nil {
			return err
		}

		for _, r := range resourcesOfType(state, "omni_dbt_environment") {
			_, err := c.GetDbtEnvironment(context.Background(), connectionID, r.Primary.ID)
			if err == nil {
				return fmt.Errorf("dbt environment %s still exists after destroy", r.Primary.ID)
			}
			if !client.IsNotFound(err) {
				return err
			}
		}
		return nil
	}
}
