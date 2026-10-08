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
    dialect = "snowflake"

    # Snowflake account identifier in <org>-<account> form, not the account
    # locator and not a full hostname. Taken from the app URL:
    # app.snowflake.com/pojcdlp/ja12247 -> pojcdlp-ja12247
    host = "pojcdlp-ja12247"

    # Objects that exist in the account. OMNI_SVC and OMNI_WH were already
    # there; ANALYTICS is created by setup-omni-svc.sql.
    database  = "ANALYTICS"
    warehouse = "OMNI_WH"
    username  = "OMNI_SVC"

    # Queried without a schema prefix.
    default_schema = "PUBLIC"

    # Omni writes upload and materialisation tables here, so it must be a
    # schema the role can write to, separate from the one it reads.
    scratch_schema = "OMNI_SCRATCH"

    # A programmatic access token is presented in the password field, so the
    # authentication type is still snowflake-password.
    authentication_type = "snowflake-password"

    # Deliberate. Role assignments resolve by priority and a permissive base
    # role outranks them, so destroying an assignment would not revoke access.
    base_role = "NO_ACCESS"

    refresh_schedule = {
      cron     = "0 2 * * ? *"
      timezone = "Australia/Melbourne"
    }
  }
}
