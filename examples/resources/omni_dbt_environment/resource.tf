resource "omni_dbt_environment" "staging" {
  connection_id = omni_connection.warehouse.id
  name          = "staging"
  target_schema = "analytics_staging"

  # Fall back to the production build for models this environment has not built
  # itself, so a partial build is still queryable.
  is_deferral_enabled = true

  # Names must start with DBT_; the API rejects anything else.
  variables = {
    DBT_LOOKBACK_DAYS = "30"
  }
}

resource "omni_dbt_environment" "developer" {
  connection_id = omni_connection.warehouse.id
  name          = "ashima-dev"
  target_schema = "dbt_ashima"

  # A personal environment, visible only to this user. Omitting owner_id makes
  # the environment shared across the organization instead.
  owner_id = omni_user.ashima.id

  target_database = "ANALYTICS_DEV"
  target_role     = "TRANSFORMER_DEV"

  variables = {
    DBT_LOOKBACK_DAYS = "7"
  }

  # Omni returns null for a secret value, so Terraform cannot detect drift on
  # these. Adding or removing a key is detected; changing a value is not.
  secret_variables = {
    DBT_EXTERNAL_API_KEY = var.dbt_external_api_key
  }
}

variable "dbt_external_api_key" {
  type        = string
  sensitive   = true
  description = "Passed to the dbt project as a secret variable."
}
