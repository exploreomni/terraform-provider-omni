# Connections. A change here is a credential or warehouse change.
#
# Secrets are not in this file. They come from connection_passwords,
# connection_private_keys and connection_oauth_secrets, passed from a secret
# store or TF_VAR_ in CI.
#
# Most tenants share embed-base. Only a tenant needing its own database or
# schema gets a connection of its own.

connections = {
  "embed-base" = {
    dialect   = "snowflake"
    host      = "your-account"
    database  = "ANALYTICS"
    warehouse = "EMBED_WH"
    username  = "OMNI_EMBED"
    base_role = "NO_ACCESS"
  }

}
