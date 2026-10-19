# User attribute definitions, and what people may hold or switch to.
#
# Three separate things, and conflating them causes real mistakes:
#
#   The definition. Created here, or referenced by ID when it was made
#   elsewhere. Create, update and delete landed in the API on 2026-09-21; before
#   that this module could only reference.
#
#   The value a person holds. Set on the user through SCIM, which is why
#   assignments resolve to a name-keyed map for modules/access rather than
#   calling an attribute endpoint. There is no group-assigned value in Omni: a
#   user's value comes from an assignment or the definition's default.
#
#   The values a person or group may switch to. A different thing again, and an
#   access-control surface. Those live in modules/selectable-values, because an
#   entry names a group and so has to run after modules/access, which itself
#   runs after this module.
#
# Definitions are referenced by ID, never by name. A definition can be renamed,
# and a configuration naming it would stop matching with no error, which on an
# access filter means a filter that silently no longer applies.

terraform {
  required_providers {
    omni = {
      source = "$(OWNER)/omni"
    }
  }
}

# Referenced rather than created. Looking it up is the point: a missing or
# mistyped ID fails at plan time instead of producing a deployment whose
# row-level security quietly does nothing.
data "omni_user_attribute" "existing" {
  for_each = { for k, v in var.attributes : k => v if v.existing_id != null }

  id = each.value.existing_id
}

# Created here. name defaults to the map key, which keeps configuration from
# carrying the same string twice.
resource "omni_user_attribute" "this" {
  for_each = { for k, v in var.attributes : k => v if v.existing_id == null }

  name  = coalesce(each.value.name, each.key)
  label = each.value.label
  type  = each.value.type

  description     = each.value.description
  multiple_values = each.value.multiple_values
  default_value   = each.value.default_value
  default_values  = each.value.default_values
}

locals {
  # One address space, whether the definition was created or looked up.
  ids = merge(
    { for k, a in data.omni_user_attribute.existing : k => a.id },
    { for k, a in omni_user_attribute.this : k => a.id },
  )

  # Local key to the attribute's current name. The SCIM payload that carries
  # values is keyed by name, so the name has to be resolved somewhere; doing it
  # here means configuration never has to mention one.
  names = merge(
    { for k, a in data.omni_user_attribute.existing : k => a.name },
    { for k, a in omni_user_attribute.this : k => a.name },
  )

  # Email to a map of attribute name to value, which is the shape
  # modules/access wants for omni_user.attributes.
  values_by_user = {
    for email, assigned in var.assignments : email => {
      for key, value in assigned : local.names[key] => value
    }
  }
}
