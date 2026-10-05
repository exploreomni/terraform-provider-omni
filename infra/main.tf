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

  connections = var.connections
  passwords   = var.connection_passwords
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
      for k, v in var.tenants : "tenant-${k}" => {
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
    for k, v in var.groups : k => merge(v, {
      model_id      = v.model == null ? null : module.modeling.model_ids[v.model]
      connection_id = v.connection == null ? null : module.warehouse.connection_ids[v.connection]
    })
  }

  folders = var.folders
}

# --------------------------------------------------------------------------
# Embed: tenants who consume content. Leave tenants empty on an internal-only
# instance.
# --------------------------------------------------------------------------

# The attribute tenant routing keys off. Fails at plan time when it is missing,
# which is the point: definitions cannot be created through the API.
data "omni_user_attribute" "tenant" {
  count = length(var.tenants) > 0 ? 1 : 0
  name  = var.tenant_attribute
}

resource "omni_folder" "tenants" {
  count = length(var.tenants) > 0 ? 1 : 0

  name = var.tenant_parent_folder
  path = lower(replace(var.tenant_parent_folder, " ", "-"))
}

module "tenant" {
  source   = "../modules/embed-tenant"
  for_each = var.tenants

  tenant_key           = each.key
  base_connection_id   = module.warehouse.connection_ids[each.value.base_connection]
  tenant_connection_id = module.warehouse.connection_ids[each.value.connection]
  parent_folder_id     = omni_folder.tenants[0].id

  content_role = each.value.content_role
  model_role   = each.value.model_role
  model_id     = each.value.model == null ? null : module.modeling.model_ids[each.value.model]
}
