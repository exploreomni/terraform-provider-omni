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
    # locator and not a hostname. From the app URL
    # app.snowflake.com/pojcdlp/ja12247 -> pojcdlp-ja12247
    host = "pojcdlp-ja12247"

    database  = "ANALYTICS"
    warehouse = "OMNI_WH"
    username  = "OMNI_SVC"

    # Omni writes upload and materialisation tables here, so it has to be a
    # schema the role can write to, separate from the ones it models.
    scratch_schema = "OMNI_SCRATCH"

    # default_schema is deliberately unset. Omni accepts it on a Snowflake
    # connection and stores null, so setting it only makes state disagree with
    # the instance. The provider documents it as required for MSSQL.

    # A programmatic access token is presented in the password field, so the
    # authentication type is still the password one.
    authentication_type = "snowflake-password"

    # Deliberate. Role assignments resolve by priority and a permissive base
    # role outranks them, so destroying an assignment would not revoke access.
    base_role = "NO_ACCESS"

    # Works once the schema model exists, which models.auto.tfvars creates.
    refresh_schedule = {
      cron     = "0 2 * * ? *"
      timezone = "Australia/Melbourne"
    }
  }
}
