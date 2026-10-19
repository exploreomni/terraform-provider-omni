package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/exploreomni/terraform-provider-omni/internal/client"
)

// A folder and a folder nested inside it.
//
// The nesting is the point. A child's path is derived from its parent's, so
// renaming the parent has to cascade: finding 12 was a child whose path went
// stale and whose next plan showed a permanent diff. The second step renames
// the parent and then plans again, which is where a broken cascade surfaces.
func TestAccFolder_nested(t *testing.T) {
	parent := accName("folder")
	child := accName("folder-child")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckFolderDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccFolderConfig(parent, child),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_folder.parent", "name", parent),
					resource.TestCheckResourceAttr("omni_folder.parent", "scope", "organization"),
					resource.TestCheckResourceAttrSet("omni_folder.parent", "path"),
					resource.TestCheckResourceAttrSet("omni_folder.parent", "url"),
					resource.TestCheckResourceAttr("omni_folder.child", "name", child),
					// The child points at the parent rather than repeating a path.
					resource.TestCheckResourceAttrPair(
						"omni_folder.child", "parent_folder_id",
						"omni_folder.parent", "id",
					),
				),
			},
			{
				Config:   testAccFolderConfig(parent, child),
				PlanOnly: true,
			},
			{
				ResourceName:      "omni_folder.parent",
				ImportState:       true,
				ImportStateVerify: true,
				// Import cannot know these: the API does not report whether a
				// folder was created with path-conflict resolution, and
				// delete_recursively is a Terraform-side choice about how to
				// destroy, not a property of the folder.
				ImportStateVerifyIgnore: []string{"resolve_path_conflict", "delete_recursively"},
			},
			{
				// Rename the parent. The child's path has to follow.
				Config: testAccFolderConfig(parent+"-renamed", child),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("omni_folder.parent", "name", parent+"-renamed"),
					resource.TestCheckResourceAttrSet("omni_folder.child", "path"),
				),
			},
			{
				// And the plan after it is empty. A stale child path shows up
				// here and nowhere earlier.
				Config:   testAccFolderConfig(parent+"-renamed", child),
				PlanOnly: true,
			},
		},
	})
}

func testAccFolderConfig(parent, child string) string {
	return fmt.Sprintf(`
resource "omni_folder" "parent" {
  name  = %[1]q
  scope = "organization"

  # These tests make throwaway folders and must not strand them.
  delete_recursively = true
}

resource "omni_folder" "child" {
  name             = %[2]q
  parent_folder_id = omni_folder.parent.id
}
`, parent, child)
}

func testAccCheckFolderDestroyed(state *terraform.State) error {
	c, err := sweepClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	for _, rs := range resourcesOfType(state, "omni_folder") {
		_, err := c.GetFolder(ctx, rs.Primary.ID, "", "")
		if err == nil {
			return fmt.Errorf("folder %q still exists after destroy", rs.Primary.ID)
		}
		if !client.IsNotFound(err) {
			return fmt.Errorf("checking folder %q was destroyed: %w", rs.Primary.ID, err)
		}
	}
	return nil
}
