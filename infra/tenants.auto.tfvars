# Embed tenants, as three independent concerns.
#
# A tenant needs a group. Everything else is optional and declared separately,
# because needing one does not imply needing the others:
#
#   tenant_groups    who the session is. The "groups" claim resolves to these.
#   tenant_routing   where its queries run. Only for physical isolation.
#   content_grants   what it can see. Usually a shared hub folder.
#
# Onboarding is one entry in tenant_groups plus one in content_grants, or one
# line if the tenant joins an existing grant.

tenant_attribute = "tenant_id"

# Who the session is.
tenant_groups = {
  acme    = { model = "embed_metrics", connection = "embed-base" }
  globex  = { model = "embed_metrics", connection = "embed-base" }
  initech = { model = "embed_metrics", connection = "embed-base" }
}

# Where its queries run.
#
# Empty for most deployments: every tenant shares embed-base and rows are
# isolated by access filters keyed off tenant_id. initech is here because its
# data sits in a separate schema.
tenant_base_connection = "embed-base"

tenant_routing = {
  initech = { connection = "embed-initech" }
}
