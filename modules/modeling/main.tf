# Models as objects, and their git connections.
#
# Model content is not managed here. Topics, views, fields, access grants and
# access filters live in model YAML and are versioned through git sync. This
# module creates the model and wires it to the repository that versions it.

terraform {
  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

# Schema models are created before every other kind, in their own resource.
#
# A connection is unusable until its schema model exists. This is what the UI's
# "Build Schema" button does, and it has no endpoint of its own: it is an
# ordinary POST /v1/models carrying the connection id and the SCHEMA kind.
# Until it exists, a shared model fails with "Schema model does not exist" and
# POST /v1/connections/{id}/schedules answers "Connection does not exist",
# which reads as a missing connection rather than a missing schema.
#
# Splitting by kind lets one apply create the connection, its schema model and
# the models built on it, in that order. A single for_each could not, because
# Terraform orders nothing within one.
resource "omni_model" "schema" {
  for_each = { for k, v in var.models : k => v if v.kind == "SCHEMA" }

  # No name. Omni names a schema model after its connection whatever is asked
  # for, and a provider may not return a value that contradicts configuration,
  # so setting one fails the apply with "inconsistent result after apply". The
  # map key stays the Terraform-side identity; the Omni-side name is the
  # connection's.
  model_kind    = each.value.kind
  connection_id = each.value.connection_id
  base_model_id = each.value.base_model_id

  # Creating a schema model registers it; introspecting the warehouse is a
  # separate call. The provider makes that call for SCHEMA models unless told
  # otherwise, so one apply leaves the connection actually usable.
  refresh_on_create = each.value.refresh_on_create
}

resource "omni_model" "this" {
  for_each = { for k, v in var.models : k => v if v.kind != "SCHEMA" }

  # The key is the model's identity; name is the label. Omni renames a model in
  # place, so a rename should be an update, not a replacement. Deriving the
  # name from the key would make every rename a destroy and create, discarding
  # the model's content and breaking everything that references the old key.
  name              = coalesce(each.value.name, each.key)
  model_kind        = each.value.kind
  connection_id     = each.value.connection_id
  base_model_id     = each.value.base_model_id
  refresh_on_create = each.value.refresh_on_create

  depends_on = [omni_model.schema]
}

locals {
  # One address space for models, whichever resource created them.
  model_ids = merge(
    { for k, m in omni_model.schema : k => m.id },
    { for k, m in omni_model.this : k => m.id },
  )
}

resource "omni_model_git" "this" {
  for_each = {
    for k, v in var.models : k => v if v.git != null
  }

  model_id    = local.model_ids[each.key]
  clone_url   = each.value.git.clone_url
  auth_method = each.value.git.auth_method

  base_branch             = each.value.git.base_branch
  branch_per_pull_request = each.value.git.branch_per_pull_request
  require_pull_request    = each.value.git.require_pull_request
  git_follower            = each.value.git.follower
  git_service_provider    = each.value.git.service_provider
  model_path              = each.value.git.model_path

  # Credentials are never returned by the API, so drift on them cannot be
  # detected, and they are stored in plain text in state.
  deploy_private_key         = try(var.git_credentials[each.key].deploy_private_key, null)
  deploy_key_passphrase      = try(var.git_credentials[each.key].deploy_key_passphrase, null)
  token                      = try(var.git_credentials[each.key].token, null)
  github_app_installation_id = each.value.git.github_app_installation_id
}
