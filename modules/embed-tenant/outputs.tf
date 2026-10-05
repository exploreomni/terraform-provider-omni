output "signed_url_values" {
  description = "Everything a signed URL for this tenant refers to."
  value = {
    user_attribute_value = var.tenant_key
    group                = omni_user_group.tenant.display_name
    content_path         = "/${omni_folder.tenant.path}"
    folder_id            = omni_folder.tenant.id
    connection_id        = var.tenant_connection_id
  }
}

output "group_id" {
  value = omni_user_group.tenant.id
}

output "folder_id" {
  value = omni_folder.tenant.id
}


