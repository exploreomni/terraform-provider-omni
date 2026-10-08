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
