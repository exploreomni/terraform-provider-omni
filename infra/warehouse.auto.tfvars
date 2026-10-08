# Connections. A change here is a credential or warehouse change.
#
# Secrets are not in this file. They come from connection_passwords,
# connection_private_keys and connection_oauth_secrets, passed from a secret
# store or TF_VAR_ in CI.

connections = {
  "embed-base" = {
    dialect = "snowflake"

    # Snowflake account identifier in <org>-<account> form, from the app URL
    # app.snowflake.com/pojcdlp/ja12247
    host = "pojcdlp-ja12247"

    database  = "ANALYTICS"
    warehouse = "OMNI_WH"
    username  = "OMNI_SVC"

    # Omni writes uploads and materialisations here, so it must be writable and
    # separate from the schemas it models.
    scratch_schema = "OMNI_SCRATCH"

    # default_schema is deliberately unset: Omni accepts it on a Snowflake
    # connection and stores null.

    authentication_type = "snowflake-password"

    # Role assignments resolve by priority and a permissive base role outranks
    # them, so destroying an assignment would not revoke access.
    base_role = "NO_ACCESS"
  }
}
