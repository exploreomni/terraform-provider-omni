resource "omni_connection_environment" "acme" {
  # The connection users query through.
  base_connection_id = omni_connection.warehouse.id

  # Where their queries actually run. Usually the same warehouse pointed at a
  # different database or schema.
  environment_connection_id = omni_connection.warehouse_acme.id

  # Values of the base connection's environment user attribute that route
  # here. For per-tenant isolation this is one tenant identifier per
  # environment.
  user_attribute_values = ["acme"]
}

resource "omni_connection_environment" "globex" {
  base_connection_id        = omni_connection.warehouse.id
  environment_connection_id = omni_connection.warehouse_globex.id
  user_attribute_values     = ["globex"]
}
