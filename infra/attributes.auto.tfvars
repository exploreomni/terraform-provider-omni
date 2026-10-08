# User attribute definitions to reference, and the values assigned to people.
#
# Definitions are made in the UI and read-only in the API, so this names the ID
# rather than the reference. "tenant_id" can be renamed in the UI; the ID below
# cannot.

user_attributes = {
  tenant = "98f49f77-2eb8-4a5c-8b97-b40690e87812"
}

# A value is set on a person or it comes from the definition's default. There is
# no third source, and in particular no group-assigned value.
user_attribute_values = {
  "ashima@omni.co" = {
    tenant = "acme"
  }
}
