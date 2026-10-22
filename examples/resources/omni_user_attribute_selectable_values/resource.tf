resource "omni_user_attribute_selectable_values" "ashima_region" {
  user_attribute_id = omni_user_attribute.region.id

  # omni_user.id is the right handle here. The API documents this field as
  # "the membership id, not the user id", which reads as a warning against it,
  # but a person has both a User row and a Membership row with different
  # UUIDs, and SCIM - which omni_user reads and writes - is keyed by the
  # membership. The User id, the one a colliding create reports in its 409,
  # is rejected with a 404.
  user_id = omni_user.ashima.id

  values = ["anz", "apac"]
}

resource "omni_user_attribute_selectable_values" "analysts_region" {
  user_attribute_id = omni_user_attribute.region.id

  # Not omni_user_group.id. This endpoint wants the group's UUID primary key,
  # while SCIM and every other group-scoped endpoint - model roles, folder and
  # document permissions - use an eight-character mini_uuid. The two are
  # generated independently, nothing converts between them, and no endpoint
  # returns the UUID, so it has to be a literal until one does. Passing a
  # mini_uuid here fails with "group_id: Invalid UUID".
  group_id = "5a7b9d67-cd43-4724-a9cd-30bf1bbbd847"

  values = ["anz", "apac", "emea"]

  # Leaving this false keeps the set a limit rather than a suggestion. True
  # lets the subject switch to any value at all, which on an attribute an
  # access filter routes on is an unbounded grant.
  allow_any_value = false
}
