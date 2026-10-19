# What a user or group may switch a user attribute to.
#
# A layer of its own rather than part of modules/user-attributes, because of
# the order things have to happen in. An entry names a group, so it runs after
# modules/access; modules/access sets attribute values on its users, so it runs
# after modules/user-attributes. Putting these resources in the attributes
# module would make that a cycle and Terraform would refuse the configuration.
#
# Not an assignment. This is the set a subject can switch between from the File
# menu of a dashboard, workbook or model; their actual value still comes from a
# direct assignment or the definition's default.
#
# Provisioning a value grants data access. Access filters and shared extensions
# commonly route on the same attribute, so letting someone switch to another
# tenant's value usually lets them read that tenant's rows.

terraform {
  required_providers {
    omni = {
      source = "$(OWNER)/omni"
    }
  }
}

resource "omni_user_attribute_selectable_values" "this" {
  for_each = var.provisions

  user_attribute_id = each.value.user_attribute_id

  group_id = each.value.group_id
  user_id  = each.value.user_id

  values          = each.value.values
  allow_any_value = each.value.allow_any_value
}
