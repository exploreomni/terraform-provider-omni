terraform {
  required_version = ">= 1.5"

  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }

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

# --------------------------------------------------------------------------
# Every module below is driven by a map that defaults to empty, so a component
# you do not configure creates nothing. An internal deployment leaves the embed
# variables unset; an embed deployment leaves the access variables unset; a
# deployment doing both sets everything.
# --------------------------------------------------------------------------

module "warehouse" {
  source = "../modules/warehouse"

  connections          = var.connections
  passwords            = var.connection_passwords
  private_keys         = var.connection_private_keys
  oauth_client_secrets = var.connection_oauth_secrets
}

module "modeling" {
  source = "../modules/modeling"

  models = {
    for k, v in var.models : k => merge(v, {
      connection_id = v.connection == null ? null : module.warehouse.connection_ids[v.connection]
    })
  }

  git_credentials = var.git_credentials
}

module "branding" {
  source = "../modules/branding"

  palettes = merge(
    var.palettes,
    {
      for k, v in var.tenant_groups : "tenant-${k}" => {
        type   = "discrete"
        colors = v.colors
      } if v.colors != null
    },
  )

  labels = var.labels
}

# --------------------------------------------------------------------------
# Internal: people who build content. Leave users, groups and folders empty on
# an instance used only for embedding.
# --------------------------------------------------------------------------

module "access" {
  source = "../modules/access"

  users = var.users

  groups = {
    for k, v in var.groups : k => {
      members         = v.members
      model_role      = v.model_role
      connection_role = v.connection_role
      model_id        = v.model == null ? null : module.modeling.model_ids[v.model]
      connection_id   = v.connection == null ? null : module.warehouse.connection_ids[v.connection]
    }
  }

  folders = var.folders
}

# --------------------------------------------------------------------------
# Embed. Three independent concerns: who the session is, where its queries run,
# and what content it can see. Each is configured separately, because a tenant
# may need one without the others.
# --------------------------------------------------------------------------

# The attribute tenant scoping keys off. Looked up rather than created:
# definitions cannot be made through the API. Only read when something uses it.
data "omni_user_attribute" "tenant" {
  count = length(var.tenant_groups) > 0 ? 1 : 0
  name  = var.tenant_attribute
}

# Who the session is. The "groups" claim in a signed URL resolves to these.
module "embed_groups" {
  source = "../modules/embed-groups"

  prefix = var.tenant_group_prefix

  tenants = {
    for k, v in var.tenant_groups : k => {
      model_id      = v.model == null ? null : module.modeling.model_ids[v.model]
      connection_id = v.connection == null ? null : module.warehouse.connection_ids[v.connection]
      model_role    = v.model_role
    }
  }
}

# Where its queries run. Empty unless a tenant needs its own database or
# schema; sharing one connection with access filters is the common case.
module "embed_routing" {
  source = "../modules/embed-routing"

  base_connection_id = var.tenant_base_connection == null ? null : module.warehouse.connection_ids[var.tenant_base_connection]

  routes = {
    for k, v in var.tenant_routing : k => {
      connection_id         = module.warehouse.connection_ids[v.connection]
      user_attribute_values = v.user_attribute_values
    }
  }
}

# What content it can see. Usually a grant on a hub folder someone else made,
# not a folder per tenant.
module "content_access" {
  source = "../modules/content-access"

  existing_folders = var.existing_folders
  managed_folders  = var.managed_folders

  grants = {
    for k, v in var.content_grants : k => {
      folder = v.folder
      role   = v.role
      group_ids = concat(
        [for g in v.tenant_groups : module.embed_groups.group_ids[g]],
        [for g in v.internal_groups : module.access.group_ids[g]],
      )
      user_ids = v.user_ids
    }
  }
}
