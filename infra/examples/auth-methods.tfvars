# One connection per authentication method, as a reference. Secrets are never
# literals here: they come from the matching map, passed from a secret store.
#
#   connection_passwords        password, or the BigQuery service account JSON
#   connection_private_keys     Snowflake key pair
#   connection_oauth_secrets    OAuth client secret

connections = {
  # Snowflake, password. The simplest case.
  "snowflake-password" = {
    dialect   = "snowflake"
    host      = "myaccount" # account identifier only, not a URL
    database  = "ANALYTICS"
    warehouse = "COMPUTE_WH"
    username  = "OMNI_SVC"
  }

  # Snowflake, key pair. The private key goes in connection_private_keys.
  "snowflake-keypair" = {
    dialect   = "snowflake"
    host      = "myaccount"
    database  = "ANALYTICS"
    warehouse = "COMPUTE_WH"
    username  = "OMNI_SVC"
  }

  # Snowflake, external OAuth.
  "snowflake-oauth" = {
    dialect                          = "snowflake"
    host                             = "myaccount"
    database                         = "ANALYTICS"
    warehouse                        = "COMPUTE_WH"
    oauth_client_id                  = "omni-client"
    external_oauth_audience          = "https://myaccount.snowflakecomputing.com"
    external_oauth_authorization_url = "https://idp.example.com/oauth2/authorize"
    external_oauth_token_url         = "https://idp.example.com/oauth2/token"
  }

  # BigQuery, service account. database is the project ID, username is the
  # service account client email, and the full service account JSON goes in
  # connection_passwords.
  "bigquery-service-account" = {
    dialect  = "bigquery"
    database = "my-gcp-project"
    region   = "us"
    username = "omni@my-gcp-project.iam.gserviceaccount.com"
  }

  # BigQuery, OAuth. The client secret goes in connection_oauth_secrets.
  "bigquery-oauth" = {
    dialect         = "bigquery"
    database        = "my-gcp-project"
    region          = "us"
    oauth_client_id = "123456789.apps.googleusercontent.com"
  }

  # BigQuery, workload identity federation. No key material at all: the
  # federated identity impersonates the service account.
  "bigquery-wif" = {
    dialect                   = "bigquery"
    database                  = "my-gcp-project"
    region                    = "us"
    authentication_type       = "bigquery-workload-identity-federation"
    wif_audience              = "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/my-pool/providers/my-provider"
    wif_service_account_email = "omni@my-gcp-project.iam.gserviceaccount.com"
  }

  # Athena, machine authentication through an assumed role.
  "athena" = {
    dialect          = "athena"
    database         = "AwsDataCatalog"
    region           = "ap-southeast-2"
    use_machine_auth = true
    aws_role_arn     = "arn:aws:iam::123456789012:role/omni"
  }
}

# Values below are placeholders. In CI these come from
# TF_VAR_connection_passwords and friends.
#
# connection_passwords = {
#   "snowflake-password"       = "..."
#   "bigquery-service-account" = file("service-account.json")
# }
#
# connection_private_keys = {
#   "snowflake-keypair" = file("omni_rsa_key.pem")
# }
#
# connection_oauth_secrets = {
#   "snowflake-oauth" = "..."
#   "bigquery-oauth"  = "..."
# }
