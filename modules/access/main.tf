# Internal access: the people who build content, as opposed to the embed
# tenants who consume it.
#
# Separate from the embed modules on purpose. These are real users with seats,
# managed through SCIM, and they need an organization API key.

terraform {
  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

resource "omni_user" "this" {
  for_each = var.users

  user_name    = each.key
  display_name = each.value.display_name
  attributes   = each.value.attributes
}

resource "omni_user_group" "this" {
  for_each = var.groups

  # The key is the identity, display_name is the label. Omni renames in place,
  # so a label change should not have to become a replacement. name_prefix
  # keeps a family of groups recognisable without burying the prefix in keys.
  display_name = coalesce(each.value.display_name, "${each.value.name_prefix}${each.key}")

  # null, not an empty list, when membership is unmanaged. The provider reads
  # that as "do not track membership", and an empty list would mean "this group
  # has no members" - which on an embed group deletes every user its sessions
  # created.
  member_ids = each.value.manage_members ? [
    for email in each.value.members : omni_user.this[email].id
  ] : null
}

# A role on one model.
#
# Filtered on grant_model_role rather than on model_id: the id is normally an
# attribute of a model created in the same run, and a for_each whose keys
# depend on an apply-time value is rejected at plan time.
resource "omni_user_group_model_role" "this" {
  for_each = {
    for k, v in var.groups : k => v if v.grant_model_role
  }

  user_group_id = omni_user_group.this[each.key].id
  model_id      = each.value.model_id
  connection_id = each.value.connection_id
  role_name     = each.value.model_role
}

# A role on the connection itself, covering every model on it. Given without a
# model_id, which is what makes it connection scoped rather than model scoped.
#
# CONNECTION_ADMIN is the usual value: it lets a group administer the
# connection and everything built on it. Grant it sparingly.
resource "omni_user_group_model_role" "connection" {
  for_each = {
    for k, v in var.groups : k => v if v.connection_role != null
  }

  user_group_id = omni_user_group.this[each.key].id
  connection_id = each.value.connection_id
  role_name     = each.value.connection_role
}

resource "omni_folder" "this" {
  for_each = var.folders

  name = each.value.name
  path = each.key

  # Stated rather than defaulted, the same as in the content-access module.
  # Scope decides who can reach the folder before the grant below applies, and
  # on an internal instance that is the difference between org-wide shared
  # content and a folder only its group can open.
  scope    = each.value.scope
  owner_id = each.value.owner_id
}

resource "omni_folder_permission" "this" {
  for_each = {
    for k, v in var.folders : k => v if length(v.groups) > 0
  }

  folder_id      = omni_folder.this[each.key].id
  role           = each.value.role
  user_group_ids = [for g in each.value.groups : omni_user_group.this[g].id]
}
