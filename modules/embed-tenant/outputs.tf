output "signed_url_values" {
  description = "Everything a signed URL for this tenant refers to."
  value = {
    user_attribute_value = var.tenant_key
    group                = omni_user_group.tenant.display_name
    content_path         = "/${omni_folder.tenant.path}"
    folder_id            = omni_folder.tenant.id
    connection_id        = coalesce(var.tenant_connection_id, var.base_connection_id)
    isolation            = var.tenant_connection_id == null ? "shared connection, access filters" : "own connection"
  }
}

output "group_id" {
  value = omni_user_group.tenant.id
}

output "folder_id" {
  value = omni_folder.tenant.id
}


