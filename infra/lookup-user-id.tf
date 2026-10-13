# Temporary. Asks the provider which ID it uses for an existing account, so an
# import block can adopt it.
#
# A person carries more than one UUID and none of them is labelled. The Omni UI
# links a user as /users/<id>. A colliding create answers 409 naming a different
# ID. The 409's ID does not resolve through GET /scim/v2/users/{id}, which is
# what omni_user reads by, so it is not the one an import needs. Rather than try
# the other one and find out on the next run, this asks the provider, and the
# plan prints the answer.
#
# Delete this file once imports.tf carries the right ID.

data "omni_user" "existing" {
  user_name = "ashima@omni.co"
}

output "existing_user_id" {
  description = "The ID omni_user reads by. Put it in infra/imports.tf, then delete this file."
  value       = data.omni_user.existing.id
}
