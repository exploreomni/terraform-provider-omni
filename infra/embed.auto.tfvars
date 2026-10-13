# Embed: where a tenant's queries run.
#
# Tenant groups are in access.auto.tfvars, as entries in var.groups with
# manage_members = false - Omni has one kind of creatable group and the only
# real difference is who owns the membership list.
#
# Which attribute scopes a tenant's rows is in attributes.auto.tfvars.

tenant_base_connection = null

# Dynamic connections: routing a tenant's queries to a connection of its own.
#
# Only for physical isolation, where tenants sit in different databases or
# schemas. Sharing one connection and isolating rows with an access filter keyed
# off tenant_attribute is the common case, and needs nothing here.
#
# Scaffolded, not tested. Two things are missing before it can be:
#
#   A second connection to route to. Routing to the only connection proves
#   nothing.
#
#   The base connection's environment user attribute, which is what the values
#   below are matched against. omni_connection does not expose that field, so it
#   has to be set in the UI, and the routing will not take effect until it is.
#
# Also note the resource cannot detect drift: the API has no read endpoint for
# connection environments, so Terraform keeps what it wrote and cannot tell you
# if someone changed it.
#
#   tenant_base_connection = "embed-base"
#
#   tenant_routing = {
#     "acme" = {
#       connection = "acme-warehouse"
#
#       # Values of the base connection's environment attribute that route here.
#       # Defaults to the key, which is the usual case.
#       user_attribute_values = null
#     }
#   }

tenant_routing = {}
