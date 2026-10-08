# Embed tenants, as three independent concerns.
#
#   tenant_groups    who the session is. The "groups" claim resolves to these.
#   tenant_routing   where its queries run. Only for physical isolation.
#   content_grants   what it can see, in content.auto.tfvars.
#
# Onboarding a tenant is one entry here plus one name in a grant.

tenant_attribute = "tenant_id"

# The key is the tenant's stable identity and is what a signed URL passes in
# "groups". To change only the visible name set display_name: re-keying
# replaces the group and loses its membership.
tenant_groups = {
  acme   = { model = "embed-test", connection = "embed-base" }
  globex = { model = "embed-test", connection = "embed-base" }
}

# Every tenant shares embed-base and rows are separated by access filters keyed
# off the tenant_id user attribute, so routing stays empty. It is only for a
# tenant needing its own database or schema.
tenant_base_connection = "embed-base"

tenant_routing = {}
