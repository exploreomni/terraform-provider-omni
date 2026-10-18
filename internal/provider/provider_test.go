package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ashima-omni/terraform-provider-omni/internal/client"
)

// Acceptance tests. These create real objects on a real Omni instance, so
// every one is gated on TF_ACC and on credentials being present.
//
//	TF_ACC=1 OMNI_BASE_URL=... OMNI_API_TOKEN=... go test ./internal/provider/ -v -timeout 60m
//
// Point them at a throwaway instance. They create, update and destroy, and a
// failed run leaves objects behind - see the sweepers at the bottom of this
// file.
//
// Some resources need something the tests cannot create cheaply, a warehouse
// connection above all. Those read an ID from the environment and skip when it
// is absent, rather than failing a run that is otherwise fine:
//
//	OMNI_TEST_CONNECTION_ID   a connection the test model roles are scoped to
//	OMNI_TEST_MODEL_ID        a model the test model roles are scoped to
//	OMNI_TEST_MEMBER_USER_ID  an existing user, for membership assertions

const (
	// Every object these tests make is named with this prefix, so a sweeper
	// can find the leftovers of a failed run and nothing else.
	accPrefix = "tfacc-"
)

var (
	// One token per run, so two runs against one instance cannot collide on a
	// name, and so a leftover is traceable to the run that made it.
	accRunID     string
	accRunIDOnce sync.Once
)

func runID() string {
	accRunIDOnce.Do(func() {
		if id, err := strconv.ParseInt(os.Getenv("GITHUB_RUN_ID"), 10, 64); err == nil {
			accRunID = shortToken(id)
			return
		}
		accRunID = shortToken(time.Now().UnixNano())
	})
	return accRunID
}

// shortToken is base 36 and at most six characters, because of accNameLimit
// below: a nanosecond timestamp is nineteen digits and spends the whole
// budget on its own.
//
// Six base-36 characters is about two billion values, taken from the low end
// of the timestamp where the digits move fastest, so two runs would have to
// start within microseconds of each other to collide.
func shortToken(n int64) string {
	if n < 0 {
		n = -n
	}
	s := strconv.FormatInt(n, 36)
	if len(s) > 6 {
		s = s[len(s)-6:]
	}
	return s
}

// The tightest name limit in the provider is a label's, at 25 characters.
// Rather than special-casing labels, every generated name fits in 25.
const accNameLimit = 25

// accName builds a name that is unique to this run and recognisable to the
// sweeper. Omni names are visible to people, so they say what they are.
//
// "tfacc-" plus a six-character token plus a separator is 13, which leaves 12
// for the suffix. Going over is a mistake in the test rather than a failure
// worth reporting through the API, so it stops here with a message saying
// what to do, instead of surfacing as "Invalid Attribute Value Length" on a
// name nobody chose by hand.
func accName(suffix string) string {
	name := fmt.Sprintf("%s%s-%s", accPrefix, runID(), suffix)
	if len(name) > accNameLimit {
		panic(fmt.Sprintf(
			"acceptance test name %q is %d characters and the limit is %d (a label's). Shorten the suffix to %d characters or fewer.",
			name, len(name), accNameLimit, accNameLimit-len(accPrefix)-len(runID())-1,
		))
	}
	return name
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"omni": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck fails the test before Terraform runs if the environment
// cannot support it. Failing here names the missing variable; failing later
// produces a provider configuration error that does not.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	if os.Getenv("OMNI_BASE_URL") == "" {
		t.Fatal("OMNI_BASE_URL must be set for acceptance tests")
	}
	if os.Getenv("OMNI_API_TOKEN") == "" && os.Getenv("OMNI_API_KEY") == "" {
		t.Fatal("OMNI_API_TOKEN (or OMNI_API_KEY) must be set for acceptance tests")
	}
}

// requireEnv skips rather than fails when an optional fixture is missing. A
// test that needs a warehouse connection cannot create one, and a run without
// one is incomplete rather than broken.
func requireEnv(t *testing.T, name string) string {
	t.Helper()

	value := os.Getenv(name)
	if value == "" {
		t.Skipf("%s is not set; skipping. Set it to run this case.", name)
	}
	return value
}

// testAccClient is the API client the CheckDestroy functions use. The provider
// under test has its own; this one is for asserting against the API directly,
// which is the only way to tell a real destroy from a reported one.
func testAccClient(t *testing.T) *client.Client {
	t.Helper()

	token := os.Getenv("OMNI_API_TOKEN")
	if token == "" {
		token = os.Getenv("OMNI_API_KEY")
	}
	c, err := client.NewClient(os.Getenv("OMNI_BASE_URL"), token)
	if err != nil {
		t.Fatalf("building an API client: %v", err)
	}
	return c
}

// resourcesOfType returns the state entries for one resource type. CheckDestroy
// receives the whole state, and walking it by type keeps each check to the
// assertion it is actually making.
func resourcesOfType(state *terraform.State, typeName string) []*terraform.ResourceState {
	var out []*terraform.ResourceState
	for _, rs := range state.RootModule().Resources {
		if rs.Type == typeName {
			out = append(out, rs)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Sweepers
//
// An acceptance test that fails mid-run leaves objects behind, and on a shared
// instance those accumulate until something collides with them. `go test
// ./internal/provider/ -sweep=all` deletes anything named with accPrefix.
//
// By prefix, not by age. A label and a user attribute carry no creation time
// in the API, so a sweeper cannot tell a leftover from something a run
// created a second ago. What keeps a sweep from deleting a live run's objects
// is the concurrency group in the nightly workflow, which is the only thing
// that does - so do not run a sweep against an instance someone else is
// testing on.
//
// Only objects whose names carry the prefix. A sweeper that deletes by type
// would empty a real instance the first time someone ran it against the wrong
// one.
//
// Groups are the gap. Sweeping needs to enumerate, and the client has
// FindGroupByDisplayName and GetGroup but no list: a leftover group's name
// contains the run ID of the run that made it, so it cannot be guessed. Until
// the client wraps GET /scim/v2/groups, delete leftover tfacc- groups by hand.
// ---------------------------------------------------------------------------

func TestMain(m *testing.M) {
	resource.TestMain(m)
}

func sweptName(name string) bool {
	return strings.HasPrefix(name, accPrefix)
}

func sweepClient() (*client.Client, error) {
	token := os.Getenv("OMNI_API_TOKEN")
	if token == "" {
		token = os.Getenv("OMNI_API_KEY")
	}
	if os.Getenv("OMNI_BASE_URL") == "" || token == "" {
		return nil, fmt.Errorf("OMNI_BASE_URL and OMNI_API_TOKEN must be set to sweep")
	}
	return client.NewClient(os.Getenv("OMNI_BASE_URL"), token)
}

func init() {
	resource.AddTestSweepers("omni_label", &resource.Sweeper{
		Name: "omni_label",
		F: func(string) error {
			c, err := sweepClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			labels, err := c.ListLabels(ctx)
			if err != nil {
				return err
			}
			for _, l := range labels {
				if !sweptName(l.Name) {
					continue
				}
				// By name: a label has no ID of its own, which is why the
				// resource's import sets id and name to the same value.
				if err := c.DeleteLabel(ctx, l.Name); err != nil {
					return fmt.Errorf("deleting label %s: %w", l.Name, err)
				}
			}
			return nil
		},
	})

	resource.AddTestSweepers("omni_color_palette", &resource.Sweeper{
		Name: "omni_color_palette",
		F: func(string) error {
			c, err := sweepClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			palettes, err := c.ListColorPalettes(ctx)
			if err != nil {
				return err
			}
			for _, p := range palettes {
				if !sweptName(p.Name) {
					continue
				}
				if err := c.DeleteColorPalette(ctx, p.ID); err != nil {
					return fmt.Errorf("deleting palette %s: %w", p.Name, err)
				}
			}
			return nil
		},
	})

	resource.AddTestSweepers("omni_user_attribute", &resource.Sweeper{
		Name: "omni_user_attribute",
		// No dependency on a selectable-values sweeper. There is no endpoint
		// that lists provisionings, so there is nothing to enumerate, and
		// deleting the attribute takes whatever was provisioned on it.
		F: func(string) error {
			c, err := sweepClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			attrs, err := c.ListUserAttributes(ctx)
			if err != nil {
				return err
			}
			for _, a := range attrs {
				// System attributes are Omni's own and have no prefix, but
				// check anyway: deleting one would take every user's value.
				if a.System || !sweptName(a.Name) {
					continue
				}
				if err := c.DeleteUserAttribute(ctx, a.ID); err != nil {
					return fmt.Errorf("deleting user attribute %s: %w", a.Name, err)
				}
			}
			return nil
		},
	})
}
