# Internal people who build content. Empty on an embed-only instance.

users = {}

# Two groups, covering the two scopes a role can have.
#
# No members yet. membership resolves emails through var.users, and an
# omni_user is a real seat, so adding people is a separate decision from
# proving the grants work.
groups = {
  # Model-scoped: a role on one model, which is the common case.
  "analysts" = {
    display_name = "Analysts"
    model        = "embed-test"
    connection   = "embed-base"
    model_role   = "QUERIER"
  }

  # Connection-scoped: no model, so the role covers every model on the
  # connection. This is the wider grant and should stay small.
  "connection-admins" = {
    display_name    = "Connection Admins"
    connection      = "embed-base"
    connection_role = "CONNECTION_ADMIN"
  }
}

folders = {}
