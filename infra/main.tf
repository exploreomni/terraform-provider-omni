terraform {
  required_version = ">= 1.5"

  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }

  # State lives in HCP Terraform. Execution mode on the workspace is set to
  # Local: GitHub Actions runs the plan and apply, HCP only stores state.
  cloud {
    organization = "ashima-poc"

    workspaces {
      name = "omni-playground"
    }
  }
}

# base_url and api_token come from OMNI_BASE_URL and OMNI_API_TOKEN, which the
# workflows populate from repository secrets.
provider "omni" {}

resource "omni_folder" "reporting" {
  name = "Reporting"
  path = "reporting"
}

resource "omni_folder" "finance" {
  name             = "Finance"
  parent_folder_id = omni_folder.reporting.id
}

resource "omni_user_group" "finance" {
  display_name = "Finance"
}
resource "omni_folder" "marketing" {
  name = "Marketing"
  path = "marketing"
}


resource "omni_user" "demo" {
  user_name    = "tf-demo@example.invalid"
  display_name = "TF Demo User"
}

resource "omni_user_group" "marketing" {
  display_name = "Marketing"
  member_ids   = [omni_user.demo.id]
}
