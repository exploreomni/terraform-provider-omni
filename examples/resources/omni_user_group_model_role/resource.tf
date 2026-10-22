resource "omni_user_group_model_role" "analysts_query_sales" {
  user_group_id = omni_user_group.analysts.id
  model_id      = omni_model.sales.id
  role_name     = "QUERIER"
}

# CONNECTION_ADMIN is granted on a connection rather than a model, so
# model_id is omitted and connection_id names the target.
resource "omni_user_group_model_role" "platform_admins" {
  user_group_id = omni_user_group.platform.id
  connection_id = omni_connection.warehouse.id
  role_name     = "CONNECTION_ADMIN"
}

resource "omni_user_group_model_role" "contractors_read_sales" {
  user_group_id = omni_user_group.contractors.id
  model_id      = omni_model.sales.id
  role_name     = "VIEWER"

  # The API has no endpoint for removing a role assignment, so destroying this
  # resource downgrades the role instead. Note that NO_ACCESS is not
  # necessarily revocation: Omni resolves roles by priority, and the
  # connection's base role outranks a destroyed direct assignment. If the
  # connection grants QUERIER to everyone, destroy leaves them a QUERIER. Set
  # the connection's base_role to NO_ACCESS if destroy must actually revoke.
  role_on_destroy = "NO_ACCESS"
}
