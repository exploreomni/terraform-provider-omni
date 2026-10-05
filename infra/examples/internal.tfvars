# An internal deployment: people build and share content. No embedding.
# Leave the tenants variable unset and nothing embed related is created.

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
}

models = {
  "finance_metrics" = {
    kind       = "SHARED_EXTENSION"
    connection = "warehouse"

    git = {
      clone_url            = "git@github.com:your-org/omni-models.git"
      base_branch          = "main"
      require_pull_request = "always"
    }
  }
}

users = {
  "analyst@example.com" = { display_name = "An Analyst" }
  "lead@example.com"    = { display_name = "A Team Lead" }
}

groups = {
  "Finance" = {
    members    = ["analyst@example.com", "lead@example.com"]
    model      = "finance_metrics"
    connection = "warehouse"
    model_role = "QUERIER"
  }

  "Finance Modelers" = {
    members    = ["lead@example.com"]
    model      = "finance_metrics"
    connection = "warehouse"
    model_role = "MODELER"
  }
}

folders = {
  "finance" = {
    name   = "Finance"
    groups = ["Finance"]
    role   = "EDITOR"
  }
}

labels = {
  "verified" = {
    color       = "#0366d6"
    description = "Reviewed and trusted content"
  }
}
