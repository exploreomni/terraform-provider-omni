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

resource "omni_folder" "managed" {
  for_each = var.managed_folders

  name               = each.value.name
  path               = each.value.parent == null ? each.key : null
  parent_folder_id   = each.value.parent == null ? null : omni_folder.managed[each.value.parent].id
  delete_recursively = each.value.delete_recursively
}

locals {
  # One address space for folders, however they were obtained.
  folder_ids = merge(
    { for k, f in data.omni_folder.existing : k => f.id },
    { for k, f in omni_folder.managed : k => f.id },
  )
}

resource "omni_folder_permission" "this" {
  for_each = var.grants

  folder_id      = local.folder_ids[each.value.folder]
  role           = each.value.role
  user_group_ids = each.value.group_ids
  user_ids       = each.value.user_ids
}
