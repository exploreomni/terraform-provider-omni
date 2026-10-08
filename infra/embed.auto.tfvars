# Embed: where a tenant's queries run.
#
# Tenant groups are in access.auto.tfvars, as entries in var.groups with
# manage_members = false - Omni has one kind of creatable group and the only
# real difference is who owns the membership list.
#
# Which attribute scopes a tenant's rows is in attributes.auto.tfvars.

tenant_base_connection = null

tenant_routing = {}
