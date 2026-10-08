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

  # A group may carry its own palette. Prefixed so a group key cannot collide
  # with a key in var.palettes.
  palettes = merge(
    var.palettes,
    {
      for k, v in var.groups : "group-${k}" => {
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
      name_prefix     = v.name_prefix
      members         = v.members
      manage_members  = v.manage_members
      model_role      = v.model_role
      connection_role = v.connection_role

      # Derived from the model name in configuration, not from the id below.
      # The name is known now; the id is not known until the model exists.
      grant_model_role = v.model != null

      model_id      = v.model == null ? null : module.modeling.model_ids[v.model]
      connection_id = v.connection == null ? null : module.warehouse.connection_ids[v.connection]
    }
  }
}

# --------------------------------------------------------------------------
# Embed. Three independent concerns: who the session is, where its queries run,
# and what content it can see. Each is configured separately, because a tenant
# may need one without the others.
# --------------------------------------------------------------------------

# The attribute tenant scoping keys off. Looked up rather than created:
# definitions cannot be made through the API.
#
# Nothing consumes this. It is an existence check: embed scoping silently
# filters nothing if the attribute is missing, and a failed data source read at
# plan time is a much better way to find that out than an empty dashboard. Read
# only when embed is actually in use, which is either a group whose membership
# the sessions own or a routed tenant.
#
# By ID when one is given, by name otherwise. The data source accepts exactly
# one of the two, so the unused one is nulled rather than left at its default.
data "omni_user_attribute" "tenant" {
  count = length([for k, v in var.groups : k if !v.manage_members]) > 0 || length(var.tenant_routing) > 0 ? 1 : 0

  id   = var.tenant_attribute_id
  name = var.tenant_attribute_id == null ? var.tenant_attribute : null
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

  # Grants name groups, so the group layer must exist first. The group_ids
  # lookup below implies this only while content_grants is non-empty; stated
  # here it holds even when it is empty.
  depends_on = [module.access]

  folders = var.folders

  grants = {
    for k, v in var.content_grants : k => {
      folder    = v.folder
      role      = v.role
      group_ids = [for g in v.groups : module.access.group_ids[g]]
      user_ids  = v.user_ids
    }
  }

  document_grants = {
    for k, v in var.document_grants : k => {
      document_id  = v.document_id
      role         = v.role
      group_ids    = [for g in v.groups : module.access.group_ids[g]]
      user_ids     = v.user_ids
      access_boost = v.access_boost
    }
  }
}
