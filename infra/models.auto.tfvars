# Models. A schema model is the connection's schema: Omni names it after the
# connection, it shares the connection's ID, and it cannot be deleted alone.

models = {
  "embed-base-schema" = {
    kind       = "SCHEMA"
    connection = "embed-base"
  }

  "embed-test" = {
    kind       = "SHARED"
    connection = "embed-base"
  }
}

# Model version control.
#
# Omni pushes and pulls the model's YAML to the repository, so this wants a repo
# of its own rather than one holding other source. Uncomment the git block on
# the model above, not here, and see docs/MODEL-GIT.md for the deploy key.
#
#   "embed-test" = {
#     connection = "embed-base"
#
#     git = {
#       clone_url = "git@github.com:ashima-omni/omni-model-playground.git"
#
#       # ssh with a deploy key, or https_token with a deploy token.
#       auth_method = "ssh"
#
#       base_branch          = "main"
#       require_pull_request = "always"
#       model_path           = ""
#
#       # follower = true makes the model read-only in Omni: the repository
#       # becomes the only way to change it. Leave it false while testing.
#       follower = false
#     }
#   }
#
# The credential itself never goes in this file. It comes from the
# GIT_CREDENTIALS secret as TF_VAR_git_credentials, keyed the same as the model:
#
#   {"embed-test":{"deploy_private_key":"-----BEGIN OPENSSH PRIVATE KEY-----\n..."}}
