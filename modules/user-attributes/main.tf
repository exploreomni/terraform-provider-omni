# User attribute values, assigned to people.
#
# Definitions are read-only in the API: there is no create, update or delete for
# an attribute itself, only GET /v1/user-attributes. They are made in the UI
# under Settings > User attributes. This module references them and assigns
# values; it cannot create them, and a missing one fails at plan time rather
# than producing a deployment whose row-level security quietly does nothing.
#
# Attributes are referenced by ID, not name. An ID is the stable handle: a
# definition can be renamed in the UI, and a configuration naming it would stop
# matching without any error, which on an access filter means a filter that no
# longer applies. Referring by ID makes a rename invisible and a deletion loud.
#
# Group assignment is not here because the API does not support it. The SCIM
# group resource carries no attribute extension, the only attribute endpoint is
# a read, and Omni's own documentation describes group "allowed user attribute
# values" as a UI setting. Values are per person until that changes.

terraform {
  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

# Resolve each referenced definition. Looking them up rather than taking the
# name on trust is the point: this is what turns a typo or a missing attribute
# into a plan-time failure.
data "omni_user_attribute" "this" {
  for_each = var.attributes

  id = each.value
}

locals {
  # Local key to the attribute's current name. The SCIM payload that carries
  # values is keyed by name, so the name has to be resolved somewhere; doing it
  # here means configuration never has to mention one.
  names = { for k, a in data.omni_user_attribute.this : k => a.name }

  # Email to a map of attribute name to value, which is the shape
  # modules/access wants for omni_user.attributes.
  values_by_user = {
    for email, assigned in var.assignments : email => {
      for key, value in assigned : local.names[key] => value
    }
  }
}
