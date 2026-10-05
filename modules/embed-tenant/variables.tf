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
  description = "The connection this tenant's sessions are routed to. Created by the warehouse module."
  type        = string
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

