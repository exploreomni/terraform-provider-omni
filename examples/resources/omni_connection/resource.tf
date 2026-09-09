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
