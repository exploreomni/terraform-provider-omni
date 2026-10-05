# Connections. A change here is a credential or warehouse change.
#
# Secrets are not in this file. They come from connection_passwords,
# connection_private_keys and connection_oauth_secrets, passed from a secret
# store or TF_VAR_ in CI.

connections = {
  "embed-base" = {
    dialect   = "snowflake"
    host      = "your-account"
    database  = "ANALYTICS"
    warehouse = "EMBED_WH"
    username  = "OMNI_EMBED"
    base_role = "NO_ACCESS"
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

  "embed-globex" = {
    dialect        = "snowflake"
    host           = "your-account"
    database       = "ANALYTICS"
    default_schema = "GLOBEX"
    warehouse      = "EMBED_WH"
    username       = "OMNI_EMBED"
    base_role      = "NO_ACCESS"
  }
}
