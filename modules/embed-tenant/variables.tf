variable "tenant_key" {
  description = "The value a signed URL passes in userAttributes, and the suffix every resource takes."
  type        = string
}

variable "base_connection_id" {
  description = "The shared connection sessions query through."
  type        = string
}

variable "parent_folder_id" {
  description = "Folder this tenant's folder sits under."
  type        = string
}

variable "tenant_connection_id" {
  description = <<-DESC
    A connection of this tenant's own, for physical isolation: their sessions
    are routed to it instead of the base connection.

    Leave null for the common case, where every tenant shares one connection
    and rows are isolated by access filters keyed off the user attribute. No
    connection environment is created then.
  DESC

  type    = string
  default = null
}

variable "content_role" {
  description = "Content role the tenant group holds on its folder. VIEWER to read, EXPLORER to drill and explore."
  type        = string
  default     = "VIEWER"

  validation {
    condition     = contains(["VIEWER", "EXPLORER", "EDITOR", "MANAGER"], var.content_role)
    error_message = "content_role must be VIEWER, EXPLORER, EDITOR or MANAGER. OWNER cannot be granted to a group."
  }
}

variable "model_id" {
  description = "Model to grant the tenant group a role on. Null to skip."
  type        = string
  default     = null
}

variable "model_role" {
  description = "Role on the model. QUERY_TOPICS is Restricted Querier, which is usually right for embed."
  type        = string
  default     = "QUERY_TOPICS"
}

