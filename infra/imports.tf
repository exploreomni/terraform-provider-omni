# Accounts and objects that already exist and have to be adopted rather than
# created.
#
# An import block does this in the pipeline. The alternative is running
# terraform import by hand against the remote state, which is a step nobody
# else can see or review, and which the apply-on-merge workflow cannot do.
#
# Remove a block once its apply has run. Leaving it is harmless, but it stops
# describing anything true.

# omni_user.Create calls SCIM directly and does not adopt an existing account.
# Without this, the apply fails with:
#
#   POST /scim/v2/users returned 409: ... user faeb3931-... already has active
#   memberships to organization ...
#
# That failure stops the access module, and content_access depends on it, so the
# folders and grants behind it are never attempted either. The plan still shows
# all of them, which is why a green plan said nothing about this.
#
# The ID below is the SCIM user ID, which is what omni_user reads by:
# GET /scim/v2/users/{id}. It is NOT the ID in the Omni UI's /users/<id> URL,
# which is the membership ID. Both are UUIDs for the same person and neither is
# labelled; the 409 above is the one place the SCIM ID appears plainly.
import {
  to = module.access.omni_user.this["ashima@omni.co"]
  id = "faeb3931-f106-4e37-bb24-8748bf43bfa5"
}
