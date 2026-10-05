# --------------------------------------------------------------------------
# Warehouse. Needed by both use cases.
# --------------------------------------------------------------------------

variable "connections" {
  description = "Connections to create, keyed by name. See modules/warehouse for the full shape."
  type        = any
  default     = {}
}

variable "connection_passwords" {
  description = "Password per connection. For BigQuery service accounts this is the full service account JSON."
  type        = map(string)
  sensitive   = true
  default     = {}
}

variable "connection_private_keys" {
  description = "RSA private key per connection, for Snowflake key pair authentication."
  type        = map(string)
  sensitive   = true
  default     = {}
}

variable "connection_oauth_secrets" {
  description = "OAuth client secret per connection."
  type        = map(string)
  sensitive   = true
  default     = {}
}

# --------------------------------------------------------------------------
# Models. Needed by both use cases.
# --------------------------------------------------------------------------

variable "models" {
  description = "Models to create, keyed by name. connection is a key from var.connections."
  type        = any
  default     = {}
}

variable "git_credentials" {
  description = "Git credentials per model, keyed the same as models."
  type        = any
  sensitive   = true
  default     = {}
}

# --------------------------------------------------------------------------
# Branding. Optional for both.
# --------------------------------------------------------------------------

variable "palettes" {
  type    = map(object({ type = string, colors = list(string) }))
  default = {}
}

variable "labels" {
  type = map(object({
    color       = optional(string)
    description = optional(string)
  }))
  default = {}
}

# --------------------------------------------------------------------------
# Internal access. Leave empty on an embed-only instance.
# --------------------------------------------------------------------------

variable "users" {
  description = "Internal users, keyed by email. Requires an organization API key."
  type = map(object({
    display_name = string
    attributes   = optional(map(string))
  }))
  default = {}
}

variable "groups" {
  description = "Internal groups, keyed by display name. model and connection are keys from var.models and var.connections."
  type = map(object({
    members    = optional(list(string), [])
    model      = optional(string)
    connection = optional(string)
    model_role = optional(string, "QUERIER")
  }))
  default = {}
}

variable "folders" {
  description = "Internal folders, keyed by path."
  type = map(object({
    name   = string
    scope  = optional(string, "organization")
    groups = optional(list(string), [])
    role   = optional(string, "EDITOR")
  }))
  default = {}
}

# --------------------------------------------------------------------------
# Embed. Leave tenants empty on an internal-only instance.
# --------------------------------------------------------------------------

variable "tenant_attribute" {
  description = "Reference of the user attribute that scopes tenants. Must already exist."
  type        = string
  default     = "tenant_id"
}

variable "tenant_parent_folder" {
  type    = string
  default = "Tenants"
}

variable "tenants" {
  description = <<-DESC
    Embed tenants. The map key is the value a signed URL passes in
    userAttributes. base_connection and connection are keys from
    var.connections.
  DESC

  type = map(object({
    base_connection = string

    # A connection of this tenant's own, for physical isolation. Omit it and
    # the tenant shares the base connection, with rows isolated by access
    # filters keyed off the user attribute. That is the common case.
    connection = optional(string)

    model           = optional(string)
    colors          = optional(list(string))
    content_role    = optional(string, "VIEWER")
    model_role      = optional(string, "QUERY_TOPICS")
  }))

  default = {}
}
