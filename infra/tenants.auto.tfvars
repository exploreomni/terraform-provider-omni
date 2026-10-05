# Embed tenants. A change here is an onboarding or an offboarding.
#
# The map key is the value a signed URL passes in userAttributes and the suffix
# every tenant resource takes. base_connection and connection are keys from
# warehouse.auto.tfvars.
#
# Adding a tenant is one entry here plus one connection in
# warehouse.auto.tfvars.

tenant_attribute = "tenant_id"

tenants = {
  acme = {
    base_connection = "embed-base"
    connection      = "embed-acme"
    model           = "embed_metrics"
    colors          = ["#1f77b4", "#ff7f0e", "#2ca02c"]
  }

  globex = {
    base_connection = "embed-base"
    connection      = "embed-globex"
    model           = "embed_metrics"
    content_role    = "EXPLORER"
  }
}
