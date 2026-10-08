# Users, groups and folders.
#
# One module for both internal people and embed tenants, because Omni has one
# kind of creatable group. What differs between them is who owns the membership
# list, which is var.groups' manage_members, not which module they live in.
#
# Creating users needs an organization API key: they are real seats managed
# through SCIM.

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

# Groups.
#
# These are NOT the groups on the Embed tab of the Groups settings page. Omni
# has two kinds, and only one can be created:
#
#   Entity groups appear under Embed, one per value of the "entity" URL
#   parameter, created by the embed session itself along with a shared folder
#   named after the entity. There is no creation endpoint, so Terraform cannot
#   make them. Creating one by name here would produce a Standard group that
#   merely shares the name - a different object, resolving differently.
#
#   Non-entity groups are what the "groups" claim in a signed URL takes; the
#   embed docs call them that, and say entity membership is handled by "entity"
#   instead. They are ordinary user groups over SCIM, and they are what this
#   module creates, for internal teams and embed tenants alike.
#
# An instance with no embed groups yet shows no tabs at all on that page, so the
# distinction stays invisible until something depends on it.
#
# One consequence for content_access: an entity's shared folder exists only
# after that entity's first session, so a grant on it has to go through a
# folder with existing_path set, and the data source fails at plan time until a
# session has created it. Terraform can neither create that folder nor wait for
# it.
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
