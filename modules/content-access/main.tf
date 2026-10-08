# Grants groups access to content folders.
#
# Two shapes, and most deployments use the first:
#
#   Grant on an existing folder. Shared content lives in a hub folder someone
#   created, and this grants tenant or internal groups access to it. Nothing is
#   created, only permissions.
#
#   Create a folder and grant on it. For content that belongs to one group and
#   would not sit in a shared hub.
#
# Separate from groups and routing on purpose: a group may need no folder at
# all, and a folder may be granted to several groups.

terraform {
  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

# Folders, looked up or created.
#
# existing_path decides which. A looked-up folder fails at plan time if it is
# missing, rather than being silently created next to the real one.
data "omni_folder" "existing" {
  for_each = { for k, v in var.folders : k => v if v.existing_path != null }

  path = each.value.existing_path
}

# Top-level and nested folders are two resources rather than one.
#
# A single resource whose parent_folder_id points at another instance of itself
# is a self-reference, which Terraform rejects outright. Splitting by depth
# removes the cycle: "child" refers to "managed", never to itself.
#
# This supports one level of nesting, which is what a hub-and-subfolder layout
# needs. A deeper tree needs a third resource, or the folders creating
# themselves by path.
resource "omni_folder" "managed" {
  for_each = {
    for k, v in var.folders : k => v if v.existing_path == null && v.parent == null
  }

  name = each.value.name
  path = each.key

  # Stated rather than defaulted. The provider falls back to "organization",
  # which is a decision about who can reach the folder before any grant is
  # applied, so it should be written down.
  scope    = each.value.scope
  owner_id = each.value.owner_id

  delete_recursively = each.value.delete_recursively
}

resource "omni_folder" "child" {
  for_each = {
    for k, v in var.folders : k => v if v.existing_path == null && v.parent != null
  }

  name             = each.value.name
  parent_folder_id = omni_folder.managed[each.value.parent].id

  # No scope: a nested folder inherits its parent's.
  delete_recursively = each.value.delete_recursively
}

locals {
  # One address space for folders, however they were obtained.
  folder_ids = merge(
    { for k, f in data.omni_folder.existing : k => f.id },
    { for k, f in omni_folder.managed : k => f.id },
    { for k, f in omni_folder.child : k => f.id },
  )
}

resource "omni_folder_permission" "this" {
  for_each = var.grants

  folder_id      = local.folder_ids[each.value.folder]
  role           = each.value.role
  user_group_ids = each.value.group_ids
  user_ids       = each.value.user_ids
}

# Grants on a single document rather than on the folder holding it.
#
# A folder grant is almost always the right tool: it covers everything in the
# folder and keeps access describable in one place. This exists for the case a
# folder grant cannot express - one dashboard shared more widely than the folder
# around it.
#
# document_id is a literal identifier, not a reference. Documents are created in
# the UI, so Terraform has nothing to resolve a name against, and the ID is the
# only stable handle.
#
# Destroying this does not revoke the grant. The documents API has no endpoint
# for that, so the provider downgrades the role to NO_ACCESS, which is subject
# to the same priority rules as a model role: a permissive grant inherited from
# the folder still applies.
resource "omni_document_permission" "this" {
  for_each = var.document_grants

  document_id    = each.value.document_id
  role           = each.value.role
  user_group_ids = each.value.group_ids
  user_ids       = each.value.user_ids
  access_boost   = each.value.access_boost
}
