output "ids" {
  description = "Every resource ID this suite created."
  value = {
    folder_parent        = omni_folder.parent.id
    folder_child         = omni_folder.child.id
    user                 = omni_user.test.id
    user_group           = omni_user_group.test.id
    model                = omni_model.extension.id
    model_yaml_file      = omni_model_yaml_file.topic.id
    user_model_role      = omni_user_model_role.test.id
    group_model_role     = omni_user_group_model_role.test.id
    connection           = try(omni_connection.test[0].id, null)
  }
}

output "computed" {
  description = "Values the API filled in, used to check reads round-trip."
  value = {
    folder_parent_path = omni_folder.parent.path
    folder_parent_url  = omni_folder.parent.url
    folder_child_path  = omni_folder.child.path
    folder_child_scope = omni_folder.child.scope
    user_active        = omni_user.test.active
    model_kind         = omni_model.extension.model_kind
    yaml_checksum      = omni_model_yaml_file.topic.checksum
  }
}

output "data_sources" {
  description = "Data source lookups, to confirm they resolve to the same objects."
  value = {
    connection_id     = data.omni_connection.existing.id
    base_model_id     = data.omni_model.base.id
    user_by_email_id  = data.omni_user.by_email.id
    group_by_name_id  = data.omni_user_group.by_name.id
    user_ids_match    = data.omni_user.by_email.id == omni_user.test.id
    group_ids_match   = data.omni_user_group.by_name.id == omni_user_group.test.id
  }
}
