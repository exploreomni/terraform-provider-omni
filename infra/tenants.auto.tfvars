tenant_attribute = "tenant_id"

tenant_groups = {
  acme   = {model = "embed_metrics", connection = "embed-base" }
  globex = {model = "embed_metrics", connection = "embed-base"}
}

tenant_base_connection = null
tenant_routing         = {}
