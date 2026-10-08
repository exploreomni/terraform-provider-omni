# Embed tenants, as three independent concerns.
#
# A tenant needs a group. Everything else is optional and declared separately,
# because needing one does not imply needing the others:
#
#   tenant_groups    who the session is. The "groups" claim resolves to these.
#   tenant_routing   where its queries run. Only for physical isolation.
#   content_grants   what it can see. Usually a shared hub folder.
#
# Onboarding a tenant is one entry here plus one in content.auto.tfvars, or a
# single name added to an existing grant.

tenant_attribute = "tenant_id"

# Who the session is, and what it can do with data.
#
# The key is the tenant's stable identity and is what a signed URL passes in
# "groups". To change only the visible name, set display_name rather than
# re-keying: re-keying replaces the group and loses its membership.
tenant_groups = {
  acme   = { model = "embed_metrics", connection = "embed-base" }
  globex = { model = "embed_metrics", connection = "embed-base" }
}

# Where queries run.
#
# Every tenant shares embed-base and rows are separated by access filters keyed
# off the tenant_id user attribute, so tenant_routing stays empty. It is only
# for a tenant that needs its own database or schema.
tenant_base_connection = "embed-base"

tenant_routing = {}
