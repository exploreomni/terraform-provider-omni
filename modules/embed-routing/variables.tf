variable "base_connection_id" {
  description = "The connection sessions query through, and route away from."
  type        = string
}

variable "routes" {
  description = <<-DESC
    Routes to create, keyed by tenant. Only tenants needing their own database
    or schema appear here; everyone else shares the base connection.

    user_attribute_values defaults to the map key. Set it where one connection
    serves several attribute values.
  DESC

  type = map(object({
    connection_id         = string
    user_attribute_values = optional(list(string))
  }))

  default = {}
}
