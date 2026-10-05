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

    refresh_schedule = optional(object({
      cron         = string
      timezone     = string
      hard_refresh = optional(bool, false)
    }))
  }))

  default = {}
}

variable "passwords" {
  description = "Password per connection, keyed the same as connections. Pass from a secret, never a literal."
  type        = map(string)
  sensitive   = true
  default     = {}
}
