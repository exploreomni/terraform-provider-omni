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

  # A model is built on a connection. The connection_id below already creates
  # this dependency, but stating it covers the case where every model in
  # var.models omits a connection and the implicit edge disappears.
  depends_on = [module.warehouse]

  models = {
    for k, v in var.models : k => merge(v, {
      connection_id = v.connection == null ? null : module.warehouse.connection_ids[v.connection]
    })
  }

  git_credentials = var.git_credentials
}

module "branding" {
  source = "../modules/branding"

  # No depends_on. Palettes and labels reference nothing else, so this layer
  # runs first and in parallel with the warehouse rather than waiting on it.

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

# Schema refresh schedules. Last of the connection-scoped resources, because a
# schedule cannot be created until the connection has a schema model, and the
# modeling module is what creates those.
module "schema_refresh" {
  source = "../modules/schema-refresh"

  depends_on = [module.modeling]

  schedules = {
    for k, v in var.connections : k => {
      connection_id = module.warehouse.connection_ids[k]
      cron          = v.refresh_schedule.cron
      timezone      = v.refresh_schedule.timezone
      hard_refresh  = try(v.refresh_schedule.hard_refresh, false)
    } if try(v.refresh_schedule, null) != null
  }
}

# User attribute values. Definitions are read-only in the API and made in the
# UI; this resolves them by ID and turns the values into the shape the access
# module wants. Values are per person: Omni has no API for assigning them to a
# group.
module "user_attributes" {
  source = "../modules/user-attributes"

  attributes  = var.user_attributes
  assignments = var.user_attribute_values
}

# --------------------------------------------------------------------------
# Internal: people who build content. Leave users, groups and folders empty on
# an instance used only for embedding.
# --------------------------------------------------------------------------

module "access" {
  source = "../modules/access"

  depends_on = [module.modeling]

  users = {
    for email, u in var.users : email => merge(u, {
      # Attributes set directly on the user win over the ones resolved from
      # var.user_attribute_values, so a one-off override needs no module change.
      attributes = merge(
        try(module.user_attributes.values_by_user[email], {}),
        coalesce(u.attributes, {}),
      )
    })
  }

  groups = {
    for k, v in var.groups : k => {
      display_name    = v.display_name
      members         = v.members
      model_role      = v.model_role
      connection_role = v.connection_role

      # Derived from the model name in configuration, not from the id below.
      # The name is known now; the id is not known until the model exists.
      grant_model_role = v.model != null

      model_id      = v.model == null ? null : module.modeling.model_ids[v.model]
      connection_id = v.connection == null ? null : module.warehouse.connection_ids[v.connection]
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

  depends_on = [module.modeling]

  prefix = var.tenant_group_prefix

  tenants = {
    for k, v in var.tenant_groups : k => {
      display_name     = v.display_name
      grant_model_role = v.model != null

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

  depends_on = [module.warehouse]

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

  # Grants name groups, so both group layers must exist first. The group_ids
  # lookups below imply this only while content_grants is non-empty; stated
  # here it holds even when it is empty.
  depends_on = [module.embed_groups, module.access]

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
