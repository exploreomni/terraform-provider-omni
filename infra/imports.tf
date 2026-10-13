# Accounts and objects that already exist and have to be adopted rather than
# created.
#
# An import block does this in the pipeline. The alternative is running
# terraform import by hand against the remote state, which is a step nobody
# else can see or review, and which the apply-on-merge workflow cannot do.
#
# Remove a block once its apply has run. Leaving it is harmless, but it stops
# describing anything true.

# omni_user.Create calls SCIM directly and does not adopt an existing account,
# so without this the apply fails with a 409 and stops the access module.
# content_access depends on that module, so the folders and grants behind it are
# never attempted either, while the plan still shows all of them.
#
# This ID came from data "omni_user", which is the only source that was right.
# A person has at least two UUIDs here and none of them is labelled:
#
#   113afa8e-...  what the provider reads by, and what the Omni UI's
#                 /users/<id> URL shows. This one.
#   faeb3931-...  what a colliding create names in its 409 message, as "user".
#                 It does not resolve through GET /scim/v2/users/{id} at all.
#
# The 409 is the more official-looking source and it is the wrong one. Ask the
# provider rather than reading an ID out of an error message.
import {
  to = module.access.omni_user.this["ashima@omni.co"]
  id = "113afa8e-6e7c-4d3a-a31e-25438e329196"
}
