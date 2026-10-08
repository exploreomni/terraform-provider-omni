# Embed tenants.
#
# Tenant groups now live in var.groups with manage_members = false, because
# Omni has one kind of group and the only real difference is who owns the
# membership list. What stays here is everything about a tenant that is not a
# group: which attribute scopes its rows, and where its queries run.

# Set tenant_attribute_id to reference the definition by ID instead. Preferred:
# a rename in the UI cannot then break scoping in silence.
tenant_attribute = "tenant_id"

tenant_base_connection = null

tenant_routing = {}
