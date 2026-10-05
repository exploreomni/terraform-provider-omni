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

    # A role on the whole connection rather than one model, usually
    # CONNECTION_ADMIN. Requires connection and ignores model.
    connection_role = optional(string)
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
# Embed. Three independent maps. Configure only what a deployment needs.
# --------------------------------------------------------------------------

variable "tenant_attribute" {
  description = "Reference of the user attribute that scopes tenants. Must already exist."
  type        = string
  default     = "tenant_id"
}

variable "tenant_group_prefix" {
  description = "Prefix for tenant group names."
  type        = string
  default     = "tenant-"
}

variable "tenant_groups" {
  description = <<-DESC
    Tenant groups, keyed by tenant. The key plus the prefix is the name a
    signed URL passes in "groups".

    Set model and connection to also grant a role on a model. Omit them and
    only the group is created.
  DESC

  type = map(object({
    model      = optional(string)
    connection = optional(string)
    model_role = optional(string, "QUERY_TOPICS")
  }))

  default = {}
}

variable "tenant_base_connection" {
  description = "Key from var.connections that sessions query through. Only needed when tenant_routing is used."
  type        = string
  default     = null
}

variable "tenant_routing" {
  description = <<-DESC
    Tenants with a connection of their own, for physical isolation. Keyed by
    tenant; connection is a key from var.connections.

    Leave empty for the common case, where every tenant shares one connection
    and rows are isolated by access filters keyed off the user attribute.
  DESC

  type = map(object({
    connection            = string
    user_attribute_values = optional(list(string))
  }))

  default = {}
}

# --------------------------------------------------------------------------
# Content access. Folders and who can see them, for tenants and internal
# groups alike.
# --------------------------------------------------------------------------

variable "existing_folders" {
  description = "Folders that already exist, keyed by a local name, valued by path. Looked up, never created."
  type        = map(string)
  default     = {}
}

variable "managed_folders" {
  description = "Folders to create, keyed by local name."
  type = map(object({
    name               = string
    parent             = optional(string)
    delete_recursively = optional(bool, false)
  }))
  default = {}
}

variable "content_grants" {
  description = <<-DESC
    Who can see which folder. folder is a key from existing_folders or
    managed_folders; tenant_groups and internal_groups are keys from
    var.tenant_groups and var.groups.
  DESC

  type = map(object({
    folder          = string
    role            = optional(string, "VIEWER")
    tenant_groups   = optional(list(string), [])
    internal_groups = optional(list(string), [])
    user_ids        = optional(list(string))
  }))

  default = {}
}
