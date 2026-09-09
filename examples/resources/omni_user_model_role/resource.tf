resource "omni_user_model_role" "blob_querier" {
  user_id       = omni_user.blob.id
  model_id      = omni_model.finance_extension.id
  connection_id = omni_connection.warehouse.id
  role_name     = "QUERIER"
}

resource "omni_user_group_model_role" "finance_modeler" {
  user_group_id = omni_user_group.finance.id
  model_id      = omni_model.finance_extension.id
  role_name     = "MODELER"
}
