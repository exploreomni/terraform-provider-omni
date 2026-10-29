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

// Two kinds of case live here, and they need different connections.
//
// The cases that apply a dbt configuration need a connection Omni can reach.
// Attaching dbt to a connection it has never connected to fails with a 500,
// so those cases use the connection named by OMNI_TEST_CONNECTION_ID and skip
// without it.
//
// That connection is shared, and the dbt resources are destructive to it: the
// configuration endpoint is a PUT that replaces any existing setup, and deleting
// a configuration deletes every environment beneath it. So before touching it,
// dbtFixtureConnection reads its dbt configuration and refuses, by skipping, if
// it holds anything other than this suite's own test repository. A leftover
// from a failed run of this suite is recognised by that URL and replaced.
//
// The cases that only plan never reach Omni, so they use a throwaway connection
// that Omni stores without testing and that is never connected to. They run
// everywhere, with no fixture at all.

// The repository URL every apply-based case configures. Nothing clones it,
// because nothing triggers a schema refresh; its job is to mark a dbt
// configuration as this suite's own.
const dbtTestRepository = "git@github.com:exploreomni/terraform-provider-omni-dbt-acc.git"

// What the resource reports when a variable name appears in both variables and
// secret_variables. Matched on the summary, which this provider writes and
// which Terraform never wraps. The detail text wraps at about 80 columns, and
// the first version of this pattern straddled a wrap and never matched.
var regexpDuplicateDbtVariable = regexp.MustCompile(`Duplicate dbt variable`)

// A dbt configuration through its whole lifecycle.
func TestAccDbtConfiguration_basic(t *testing.T) {
	connection := dbtFixtureConnection(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDbtConfigurationDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccDbtConfigurationConfig(connection, "main", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "branch", "main"),
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "auth_method", "ssh"),
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "enable_virtual_schemas", "true"),
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "supports_dbt", "true"),
					// Not set in the configuration; the API supplies it. Before
					// this attribute was Computed, the mismatch failed every apply.
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "dbt_version", "Auto"),
					// The configuration is a property of the connection, so its
					// id is the connection id rather than one of its own.
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "id", connection),
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
				Config:   testAccDbtConfigurationConfig(connection, "main", true),
				PlanOnly: true,
			},
			{
				Config: testAccDbtConfigurationConfig(connection, "develop", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "branch", "develop"),
					resource.TestCheckResourceAttr("omni_dbt_configuration.test", "enable_virtual_schemas", "false"),
				),
			},
			{
				ResourceName:  "omni_dbt_configuration.test",
				ImportState:   true,
				ImportStateId: connection,
				// token is write-only and never returned, so an imported
				// configuration cannot carry it. Verifying state would compare
				// against a value the API will not supply.
				ImportStateVerify: false,
			},
		},
	})
}

// An environment, including the variable path that is easiest to get wrong.
//
// A variable already on the environment has to be sent by id on update, or the
// API adds a second variable with the same name; and it has to be sent without
// its name, which the update endpoint rejects beside an id. The provider does
// not hold the ids in state but resolves them from the API by name, so the
// third step asserts the count the API holds: a duplicate would leave two,
// while Terraform state would show one either way.
func TestAccDbtEnvironment_variablesUpdateInPlace(t *testing.T) {
	connection := dbtFixtureConnection(t)
	name := accIdentifier("dbtenv")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDbtEnvironmentDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccDbtEnvironmentConfig(connection, name, "analytics_staging", `DBT_LOOKBACK_DAYS = "30"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "name", name),
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "target_schema", "analytics_staging"),
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "variables.DBT_LOOKBACK_DAYS", "30"),
					resource.TestCheckResourceAttrSet("omni_dbt_environment.test", "id"),
					// The first environment on a configuration becomes its default.
					// That matters for destroy: Omni refuses to delete a default on
					// its own, so this case's teardown exercises the path that drops
					// it from state with a warning and leaves the configuration
					// delete to remove it.
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "is_default_environment", "true"),
				),
			},
			{
				Config:   testAccDbtEnvironmentConfig(connection, name, "analytics_staging", `DBT_LOOKBACK_DAYS = "30"`),
				PlanOnly: true,
			},
			{
				Config: testAccDbtEnvironmentConfig(connection, name, "analytics_staging", `DBT_LOOKBACK_DAYS = "7"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "variables.DBT_LOOKBACK_DAYS", "7"),
					resource.TestCheckResourceAttr("omni_dbt_environment.test", "variables.%", "1"),
					checkDbtVariableCount("DBT_LOOKBACK_DAYS", 1),
				),
			},
		},
	})
}

// A secret variable, which the API never reads back.
//
// The value returns as null, so the provider deliberately does not refresh
// variable values from a response. If it did, the null would overwrite what
// the configuration set and every plan after the first would show a diff. The
// PlanOnly step is the whole point of this case.
func TestAccDbtEnvironment_secretVariableHasNoPerpetualDiff(t *testing.T) {
	connection := dbtFixtureConnection(t)
	name := accIdentifier("dbtsec")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDbtEnvironmentDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccDbtEnvironmentSecretConfig(connection, name, "s3cret"),
				Check: resource.TestCheckResourceAttr(
					"omni_dbt_environment.test", "secret_variables.DBT_EXTERNAL_API_KEY", "s3cret"),
			},
			{
				Config:   testAccDbtEnvironmentSecretConfig(connection, name, "s3cret"),
				PlanOnly: true,
			},
		},
	})
}

// A name set in both maps is rejected before any request, since Omni stores a
// variable as secret or not and cannot hold it as both.
func TestAccDbtEnvironment_variableCannotBeBothSecretAndNot(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDbtDummyConnectionConfig(accName("dbtclash")) + fmt.Sprintf(`
resource "omni_dbt_environment" "test" {
  connection_id = omni_connection.test.id
  name          = %[1]q
  target_schema = "analytics_clash"

  variables = {
    DBT_SHARED = "plain"
  }

  secret_variables = {
    DBT_SHARED = "secret"
  }
}
`, accIdentifier("dbtclash")),
				PlanOnly:    true,
				ExpectError: regexpDuplicateDbtVariable,
			},
		},
	})
}

// Variable names must start with DBT_ and use only letters, digits and
// underscores. The API enforces that with a 400 on create, and the reference
// documentation does not mention it, so the provider checks it at plan time
// instead of failing partway through an apply.
func TestAccDbtEnvironment_variableNameMustStartWithDBT(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccDbtDummyConnectionConfig(accName("dbtname")) + testAccDbtEnvironmentConfig("omni_connection.test.id", accIdentifier("dbtname"), "analytics_names", `lookback_days = "30"`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`must start with DBT_`),
			},
		},
	})
}

// testAccDbtDummyConnectionConfig is a connection Omni will store but never
// reach. Only plan-time cases use it. Snowflake supports dbt, and the API
// requires warehouse for it.
func testAccDbtDummyConnectionConfig(name string) string {
	return fmt.Sprintf(`
resource "omni_connection" "test" {
  name      = %[1]q
  dialect   = "snowflake"
  host      = "tfacc-dummy"
  database  = "DUMMY"
  warehouse = "DUMMY_WH"
  username  = "DUMMY"
  password  = "dummy"
}
`, name)
}

// connectionExpression turns a fixture's literal id into a quoted HCL string,
// and passes a reference such as omni_connection.test.id through unchanged.
func connectionExpression(connection string) string {
	if strings.Contains(connection, ".") {
		return connection
	}
	return fmt.Sprintf("%q", connection)
}

func testAccDbtConfigurationConfig(connection, branch string, virtualSchemas bool) string {
	return fmt.Sprintf(`
resource "omni_dbt_configuration" "test" {
  connection_id = %[1]s

  ssh_url = %[2]q
  branch  = %[3]q

  auth_method            = "ssh"
  enable_virtual_schemas = %[4]t
  autogen_relationships  = false
}
`, connectionExpression(connection), dbtTestRepository, branch, virtualSchemas)
}

// An environment needs the configuration in place, and enable_virtual_schemas
// is what lets an environment route to another schema at all.
func testAccDbtEnvironmentConfig(connection, name, schema, variables string) string {
	return testAccDbtConfigurationConfig(connection, "main", true) + fmt.Sprintf(`
resource "omni_dbt_environment" "test" {
  connection_id = omni_dbt_configuration.test.connection_id
  name          = %[1]q
  target_schema = %[2]q

  variables = {
    %[3]s
  }
}
`, name, schema, variables)
}

func testAccDbtEnvironmentSecretConfig(connection, name, secret string) string {
	return testAccDbtConfigurationConfig(connection, "main", true) + fmt.Sprintf(`
resource "omni_dbt_environment" "test" {
  connection_id = omni_dbt_configuration.test.connection_id
  name          = %[1]q
  target_schema = "analytics_secret"

  secret_variables = {
    DBT_EXTERNAL_API_KEY = %[2]q
  }
}
`, name, secret)
}

// dbtFixtureConnection returns the connection the apply-based cases use, or
// skips the test when there is none, or when using it could destroy something
// this suite did not create.
func dbtFixtureConnection(t *testing.T) string {
	t.Helper()
	connectionID := requireEnv(t, "OMNI_TEST_CONNECTION_ID")

	c, err := sweepClient()
	if err != nil {
		t.Fatalf("building a client to check the dbt fixture: %v", err)
	}
	existing, err := c.GetDbtConfiguration(context.Background(), connectionID)
	if err != nil {
		t.Fatalf("reading dbt on fixture connection %s: %v", connectionID, err)
	}
	if !existing.SupportsDbt {
		t.Skipf("connection %s does not support dbt, so OMNI_TEST_CONNECTION_ID cannot run the dbt cases", connectionID)
	}
	if existing.Configured() && existing.SSHURL != dbtTestRepository {
		t.Skipf("connection %s already has dbt configured against %s. These cases replace and then "+
			"delete a dbt configuration, taking every environment beneath it, so they refuse to run "+
			"against one this suite did not create. Point OMNI_TEST_CONNECTION_ID at a connection "+
			"without dbt.", connectionID, existing.SSHURL)
	}
	return connectionID
}

// dbtStateConnectionID returns the connection the dbt configuration in state is
// attached to, for checks that query the API directly.
func dbtStateConnectionID(state *terraform.State) (string, error) {
	configurations := resourcesOfType(state, "omni_dbt_configuration")
	if len(configurations) != 1 {
		return "", fmt.Errorf("expected one omni_dbt_configuration in state, found %d", len(configurations))
	}
	return configurations[0].Primary.Attributes["connection_id"], nil
}

// checkDbtVariableCount asserts how many variables of a given name the API
// holds, which is what catches a duplicate that Terraform state would not
// show: state carries one map entry either way.
func checkDbtVariableCount(variableName string, want int) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		connectionID, err := dbtStateConnectionID(state)
		if err != nil {
			return err
		}
		environments := resourcesOfType(state, "omni_dbt_environment")
		if len(environments) != 1 {
			return fmt.Errorf("expected one omni_dbt_environment in state, found %d", len(environments))
		}

		c, err := sweepClient()
		if err != nil {
			return err
		}
		environment, err := c.GetDbtEnvironment(context.Background(), connectionID, environments[0].Primary.ID)
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

// A destroyed configuration leaves the connection reporting dbt as not
// configured. That is a 200 with a message rather than a 404, which is why this
// checks Configured rather than treating an error as the pass.
func testAccCheckDbtConfigurationDestroyed(state *terraform.State) error {
	connectionID, err := dbtStateConnectionID(state)
	if err != nil {
		return err
	}
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

func testAccCheckDbtEnvironmentDestroyed(state *terraform.State) error {
	connectionID, err := dbtStateConnectionID(state)
	if err != nil {
		return err
	}
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
