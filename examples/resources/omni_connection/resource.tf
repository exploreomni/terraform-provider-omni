resource "omni_connection" "warehouse" {
  name    = "Production Snowflake"
  dialect = "snowflake"

  host      = "myaccount"
  database  = "ANALYTICS"
  warehouse = "COMPUTE_WH"
  username  = "OMNI_SVC"

  # Rotated in place: changing this does not replace the connection.
  password = var.snowflake_password

  base_role       = "QUERIER"
  include_schemas = "PUBLIC,ANALYTICS"

  query_timeout_seconds = 900
}

variable "snowflake_password" {
  type      = string
  sensitive = true
}

# A base connection for dynamic environments. The attribute named here routes a
# query to one of the connection's environments based on the value the querying
# user holds; the default values are the ones that stay on this connection.
#
# Set this here rather than in the UI, then attach the environments with
# omni_connection_environment. Note the API takes these two together, so
# removing one without the other is rejected at plan time.
resource "omni_connection" "tenant_base" {
  name    = "tenant-base"
  dialect = "snowflake"

  host      = "myaccount"
  database  = "ANALYTICS"
  warehouse = "COMPUTE_WH"
  username  = "OMNI_SVC"
  password  = var.snowflake_password

  environment_user_attribute_name           = omni_user_attribute.tenant_id.name
  environment_user_attribute_default_values = ["internal"]
}
