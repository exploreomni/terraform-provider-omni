# Embed tenants. A change here is an onboarding or an offboarding.
#
# The map key is the value a signed URL passes in userAttributes and the suffix
# every tenant resource takes.
#
# Two isolation models, and you can mix them:
#
#   Shared connection, the common case. Omit "connection". Every tenant queries
#   the base connection and rows are isolated by access filters in model YAML
#   keyed off the user attribute. Nothing extra is created.
#
#   Own connection, for physical isolation. Set "connection" to a key from
#   warehouse.auto.tfvars. A connection environment routes that tenant's
#   sessions to it, so tenants sit in different databases or schemas.

tenant_attribute = "tenant_id"

tenants = {
  # Shared connection with access filters.
  acme = {
    base_connection = "embed-base"
    model           = "embed_metrics"
    colors          = ["#1f77b4", "#ff7f0e", "#2ca02c"]
  }

  # Also shared.
  globex = {
    base_connection = "embed-base"
    model           = "embed_metrics"
    content_role    = "EXPLORER"
  }

  # Physically isolated: its own connection and schema.
  initech = {
    base_connection = "embed-base"
    connection      = "embed-initech"
    model           = "embed_metrics"
  }
}
