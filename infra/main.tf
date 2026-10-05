terraform {
  required_version = ">= 1.5"

  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }

  # A new workspace, not the old one. Reusing omni-playground would start from
  # state describing a different instance.
  cloud {
    organization = "ashima-poc"

    workspaces {
      name = "omni-terraform"
    }
  }
}

# base_url and api_token come from OMNI_BASE_URL and OMNI_API_TOKEN, which the
# workflows populate from repository secrets.
provider "omni" {}
