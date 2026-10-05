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

resource "omni_model" "this" {
  for_each = var.models

  name          = each.key
  model_kind    = each.value.kind
  connection_id = each.value.connection_id
  base_model_id = each.value.base_model_id
}

resource "omni_model_git" "this" {
  for_each = {
    for k, v in var.models : k => v if v.git != null
  }

  model_id   = omni_model.this[each.key].id
  clone_url  = each.value.git.clone_url
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
