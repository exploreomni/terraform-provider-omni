# An embed deployment: tenants consume content through signed URLs. No internal
# users. Leave users, groups and folders unset and nothing internal is created.
#
# The tenant_id attribute must already exist on the instance. Definitions
# cannot be created through the API.

connections = {
  "embed-base" = {
    dialect   = "snowflake"
    host      = "your-account"
    database  = "ANALYTICS"
    warehouse = "EMBED_WH"
    username  = "OMNI_EMBED"
    base_role = "NO_ACCESS"
  }

  "embed-acme" = {
    dialect        = "snowflake"
    host           = "your-account"
    database       = "ANALYTICS"
    default_schema = "ACME"
    warehouse      = "EMBED_WH"
    username       = "OMNI_EMBED"
    base_role      = "NO_ACCESS"
  }

  "embed-globex" = {
    dialect        = "snowflake"
    host           = "your-account"
    database       = "ANALYTICS"
    default_schema = "GLOBEX"
    warehouse      = "EMBED_WH"
    username       = "OMNI_EMBED"
    base_role      = "NO_ACCESS"
  }
}

models = {
  "embed_metrics" = {
    kind       = "SHARED_EXTENSION"
    connection = "embed-base"
  }
}

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
