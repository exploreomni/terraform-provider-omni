# Routes a tenant's sessions to a connection of their own.
#
# Only needed for physical isolation, where tenants sit in different databases
# or schemas. Most deployments share one connection and isolate rows with
# access filters keyed off the user attribute, in which case this module is
# left empty and creates nothing.
#
# Separate from groups on purpose: routing is about where queries run, group
# membership is about who the session is. Neither implies the other.

terraform {
  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

# This resource cannot detect drift: the API has no read endpoint for
# connection environments. Terraform keeps what it wrote and cannot tell you if
# someone changed it in the UI.
resource "omni_connection_environment" "tenant" {
  for_each = var.routes

  base_connection_id        = var.base_connection_id
  environment_connection_id = each.value.connection_id

  # Values of the base connection's environment user attribute that route here.
  # Defaults to the map key, which is the usual case: one tenant, one value.
  user_attribute_values = coalesce(each.value.user_attribute_values, [each.key])
}
