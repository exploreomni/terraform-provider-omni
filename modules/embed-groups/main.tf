# Tenant groups, and what they can do with data.
#
# A group is what the "groups" claim in a signed URL resolves to. Creating one
# implies nothing about folders or routing: those are separate concerns and
# separate modules.

terraform {
  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

resource "omni_user_group" "tenant" {
  for_each = var.tenants

  display_name = "${var.prefix}${each.key}"

  # Deliberately no member_ids. Embed sessions create their own users, and
  # membership here is authoritative, so Terraform would remove whoever the
  # session added.
}

# Optional: what the group can do with data. Skipped for a tenant with no
# model, which is the right default when access comes from a shared grant
# elsewhere.
#
# The filter is on grant_model_role, not on model_id. A model_id usually comes
# from a model Terraform is creating in the same run, so it is unknown until
# apply, and Terraform refuses a for_each whose set of keys it cannot work out
# at plan time. grant_model_role is set from configuration, so the keys are
# known and only the values arrive late.
resource "omni_user_group_model_role" "tenant" {
  for_each = {
    for k, v in var.tenants : k => v if v.grant_model_role
  }

  user_group_id = omni_user_group.tenant[each.key].id
  model_id      = each.value.model_id
  connection_id = each.value.connection_id
  role_name     = each.value.model_role
}
