# Models and their git connections.
#
# Two models, created in order by the modeling module:
#
#   embed-base-schema  the connection's schema model. This is what the UI's
#                      "Build Schema" button creates, and it has no endpoint of
#                      its own: it is POST /v1/models carrying the connection id
#                      and the SCHEMA kind. A connection is unusable until it
#                      exists, and both a shared model and a refresh schedule
#                      fail with errors naming the connection rather than the
#                      missing schema.
#
#                      The key is the Terraform-side identity only. Omni names a
#                      schema model after its connection, so this one is called
#                      "embed-base" in the UI, and setting name here would fail
#                      the apply.
#
#   embed-test         the shared model built on it. This one is named by its
#                      key, or by name if you set one.
#
# The module creates schema models ahead of every other kind, and the provider
# starts a schema refresh after creating one, so a single apply leaves the
# connection usable. Set refresh_on_create = false on a model to skip that.

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
