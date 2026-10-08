variable "connections" {
  description = <<-DESC
    Connections to create, keyed by name. Only credentials and base_role can be
    changed in place; every other field forces replacement, which takes the
    models built on the connection with it.
  DESC

  type = map(object({
    dialect  = string
    host     = optional(string)
    port     = optional(number)
    database = optional(string)
    username = optional(string)

    warehouse      = optional(string)
    region         = optional(string)
    default_schema = optional(string)
    scratch_schema = optional(string)

    include_schemas       = optional(string)
    query_timeout_seconds = optional(number)
    base_role             = optional(string, "NO_ACCESS")

    # Authentication, beyond a plain username and password.
    authentication_type = optional(string)
    oauth_client_id     = optional(string)
    use_machine_auth    = optional(bool)
    aws_role_arn        = optional(string)
    host_override       = optional(string)

    wif_audience              = optional(string)
    wif_service_account_email = optional(string)

    external_oauth_audience          = optional(string)
    external_oauth_authorization_url = optional(string)
    external_oauth_token_url         = optional(string)

    # A schema refresh schedule.
    #
    # Requires the connection's schema model to exist first. Until it does,
    # POST /v1/connections/{id}/schedules answers "Connection with id ... does
    # not exist", even though GET and PATCH on the connection return 200. So a
    # 404 here means "no schema model yet", not "no connection".
    #
    # Create the schema model with a kind = "SCHEMA" entry in var.models, which
    # is the API equivalent of the UI's "Build Schema" button.
    refresh_schedule = optional(object({
      cron         = string
      timezone     = string
      hard_refresh = optional(bool, false)
    }))
  }))

  default = {}
}

variable "passwords" {
  description = <<-DESC
    Password per connection, keyed the same as connections. For BigQuery with a
    service account this is the full service account JSON, not a password.
    Pass from a secret, never a literal.
  DESC

  type      = map(string)
  sensitive = true
  default   = {}
}

variable "private_keys" {
  description = "RSA private key per connection, PEM format, for Snowflake key pair authentication."
  type        = map(string)
  sensitive   = true
  default     = {}
}

variable "oauth_client_secrets" {
  description = "OAuth client secret per connection, for Snowflake or BigQuery OAuth."
  type        = map(string)
  sensitive   = true
  default     = {}
}
