# Models. The schema model is the connection's schema: Omni names it after
# the connection and it cannot be deleted on its own.

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
