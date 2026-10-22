resource "omni_dbt_configuration" "warehouse" {
  connection_id = omni_connection.warehouse.id

  ssh_url = "git@github.com:acme/dbt-analytics.git"
  branch  = "main"

  auth_method = "ssh"

  # Required for dbt environments to route queries to a different schema.
  enable_virtual_schemas = true

  # Generate Omni relationships from the relationships dbt already declares,
  # rather than redefining them in the semantic model.
  autogen_relationships = true

  committer_name  = "Omni"
  committer_email = "data-platform@acme.example"
}

# With auth_method = "ssh", Omni generates the deploy key. It has to be added to
# the repository before the first schema refresh, or Omni cannot clone it.
output "omni_deploy_key" {
  value       = omni_dbt_configuration.warehouse.public_key
  description = "Add to the dbt repository as a deploy key."
}

resource "omni_dbt_configuration" "monorepo" {
  connection_id = omni_connection.staging.id

  ssh_url = "https://github.com/acme/platform.git"
  branch  = "main"

  auth_method = "https_token"
  token       = var.dbt_repository_token

  # dbt_project.yml is not at the repository root.
  project_root_path = "warehouse/dbt"

  dbt_version = "1.11"
}

variable "dbt_repository_token" {
  type      = string
  sensitive = true

  # The API never returns this, so Terraform cannot detect drift on it.
  description = "Deploy token for the dbt repository."
}
