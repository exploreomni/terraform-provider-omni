# Warehouse connections and their schema refresh schedules.
#
# Used by every deployment, internal or embedded. An embed deployment adds
# per-tenant connections through the embed-tenant module and routes to them.

terraform {
  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

resource "omni_connection" "this" {
  for_each = var.connections

  name     = each.key
  dialect  = each.value.dialect
  host     = each.value.host
  port     = each.value.port
  database = each.value.database
  username = each.value.username
  password = var.passwords[each.key]

  warehouse      = each.value.warehouse
  region         = each.value.region
  default_schema = each.value.default_schema
  scratch_schema = each.value.scratch_schema

  # Authentication. Which of these apply depends on the dialect and the method:
  #
  #   Snowflake, password        password
  #   Snowflake, key pair        private_key, username
  #   Snowflake, external OAuth  oauth_client_id, oauth_client_secret,
  #                              external_oauth_* and authentication_type
  #   BigQuery, service account  username is the client email, password is the
  #                              full service account JSON, database is the
  #                              project ID
  #   BigQuery, OAuth            oauth_client_id, oauth_client_secret
  #   Athena, Databricks         use_machine_auth, aws_role_arn
  authentication_type = each.value.authentication_type
  private_key         = try(var.private_keys[each.key], null)
  oauth_client_id     = each.value.oauth_client_id
  oauth_client_secret = try(var.oauth_client_secrets[each.key], null)
  use_machine_auth    = each.value.use_machine_auth
  aws_role_arn        = each.value.aws_role_arn
  host_override       = each.value.host_override

  wif_audience              = each.value.wif_audience
  wif_service_account_email = each.value.wif_service_account_email

  external_oauth_audience          = each.value.external_oauth_audience
  external_oauth_authorization_url = each.value.external_oauth_authorization_url
  external_oauth_token_url         = each.value.external_oauth_token_url

  include_schemas       = each.value.include_schemas
  query_timeout_seconds = each.value.query_timeout_seconds

  # The access floor. NO_ACCESS means access is granted per model, which is
  # what makes a role assignment meaningful: destroying one downgrades it to
  # NO_ACCESS, and a permissive base role would win instead.
  base_role = each.value.base_role
}

resource "omni_connection_schedule" "this" {
  for_each = {
    for k, v in var.connections : k => v if v.refresh_schedule != null
  }

  connection_id = omni_connection.this[each.key].id

  # Six-field EventBridge cron: minute hour day-of-month month day-of-week year.
  # Daily at 02:00 is "0 2 * * ? *".
  schedule     = each.value.refresh_schedule.cron
  timezone     = each.value.refresh_schedule.timezone
  hard_refresh = each.value.refresh_schedule.hard_refresh
}
