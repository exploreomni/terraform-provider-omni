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

# Folders that already exist. Looked up rather than created, so a missing one
# fails at plan time instead of being silently recreated.
data "omni_folder" "existing" {
  for_each = var.existing_folders

  path = each.value
}

# Top-level folders and nested ones are two resources rather than one.
#
# A single resource whose parent_folder_id points at another instance of itself
# is a self-reference, which Terraform rejects outright. Splitting by depth
# removes the cycle: "child" refers to "managed", never to itself.
#
# This supports one level of nesting, which is what a hub-and-subfolder layout
# needs. A deeper tree needs a third resource, or the folders creating
# themselves by path.
resource "omni_folder" "managed" {
  for_each = { for k, v in var.managed_folders : k => v if v.parent == null }

  name = each.value.name
  path = each.key

  # Stated rather than defaulted. The provider falls back to "organization",
  # which on an embed instance is a decision about who can reach the folder
  # before any grant is applied, so it should be written down.
  scope    = each.value.scope
  owner_id = each.value.owner_id

  delete_recursively = each.value.delete_recursively
}

resource "omni_folder" "child" {
  for_each = { for k, v in var.managed_folders : k => v if v.parent != null }

  name               = each.value.name
  parent_folder_id   = omni_folder.managed[each.value.parent].id
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
