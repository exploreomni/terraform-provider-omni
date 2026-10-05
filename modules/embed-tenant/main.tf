# Everything one embed tenant needs: an isolated connection, the routing that
# sends their sessions to it, a group, a folder and the permission on it.
#
# The embed entity itself cannot be created here. There is no API for it, so a
# person creates it in the UI. See docs/API-COVERAGE.md.

terraform {
  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

# Routes a session carrying the tenant key to that connection.
#
# This resource cannot detect drift: the API has no read endpoint for
# connection environments.
resource "omni_connection_environment" "tenant" {
  base_connection_id        = var.base_connection_id
  environment_connection_id = var.tenant_connection_id
  user_attribute_values     = [var.tenant_key]
}

# Matches the "groups" claim in the signed URL.
#
# Deliberately no member_ids: embed sessions create their own users, and
# membership here is authoritative, so Terraform would remove them on the next
# apply.
resource "omni_user_group" "tenant" {
  display_name = "tenant-${var.tenant_key}"
}

# What contentPath points at.
resource "omni_folder" "tenant" {
  name             = var.tenant_key
  parent_folder_id = var.parent_folder_id

  # Tenant folders hold tenant content, so a destroy has to take it with them.
  delete_recursively = true
}

resource "omni_folder_permission" "tenant" {
  folder_id      = omni_folder.tenant.id
  role           = var.content_role
  user_group_ids = [omni_user_group.tenant.id]
}

# Optional: what the tenant group can do with data, where a model is given.
#
# Destroying this does not revoke access, it downgrades to NO_ACCESS. That is
# only equivalent because the connections above use NO_ACCESS as their base
# role.
resource "omni_user_group_model_role" "tenant" {
  count = var.model_id == null ? 0 : 1

  user_group_id = omni_user_group.tenant.id
  model_id      = var.model_id
  connection_id = var.base_connection_id
  role_name     = var.model_role
}
