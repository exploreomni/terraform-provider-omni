# Both: an internal team builds the content that embed tenants consume.
#
# This is the common shape. Internal users model and build on the shared
# connection; tenants see the results through isolated connections.

connections = {
  "warehouse" = {
    dialect   = "snowflake"
    host      = "your-account"
    database  = "ANALYTICS"
    warehouse = "COMPUTE_WH"
    username  = "OMNI_SVC"
    base_role = "NO_ACCESS"

    refresh_schedule = {
      cron     = "0 2 * * ? *"
      timezone = "Australia/Melbourne"
    }
  }

  "embed-acme" = {
    dialect        = "snowflake"
    host           = "your-account"
    database       = "ANALYTICS"
    default_schema = "ACME"
    warehouse      = "EMBED_WH"
    username       = "OMNI_EMBED"
    base_role      = "NO_ACCESS"
  }
}

models = {
  "product_metrics" = {
    kind       = "SHARED_EXTENSION"
    connection = "warehouse"

    git = {
      clone_url            = "git@github.com:your-org/omni-models.git"
      require_pull_request = "always"
    }
  }
}

users = {
  "analyst@example.com" = { display_name = "An Analyst" }
}

groups = {
  "Analytics" = {
    members    = ["analyst@example.com"]
    model      = "product_metrics"
    connection = "warehouse"
    model_role = "MODELER"
  }
}

folders = {
  "internal" = {
    name   = "Internal"
    groups = ["Analytics"]
    role   = "EDITOR"
  }
}

tenants = {
  acme = {
    base_connection = "warehouse"
    connection      = "embed-acme"
    model           = "product_metrics"
  }
}
