terraform {
  required_providers {
    omni = {
      source  = "ashima-omni/omni"
      version = "~> 0.1"
    }
  }
}

provider "omni" {
  # Both can be supplied by OMNI_BASE_URL and OMNI_API_TOKEN instead.
  base_url  = "https://blobsrus.omniapp.co"
  api_token = var.omni_api_token
}

variable "omni_api_token" {
  type      = string
  sensitive = true
}
