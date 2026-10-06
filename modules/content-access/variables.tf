variable "existing_folders" {
  description = "Folders that already exist, keyed by a local name, valued by path. Looked up, never created."
  type        = map(string)
  default     = {}
}

variable "managed_folders" {
  description = "Folders to create, keyed by path or local name. parent refers to another key in this map."
  type = map(object({
    name               = string
    parent             = optional(string)
    delete_recursively = optional(bool, false)
  }))
  default = {}

  validation {
    condition = alltrue([
      for k, v in var.managed_folders :
      v.parent == null || contains(keys(var.managed_folders), v.parent)
    ])
    error_message = "Every parent must be another key in managed_folders."
  }

  validation {
    # One level of nesting. A child of a child would fail later with an
    # unhelpful index error, so it is caught here instead.
    condition = alltrue([
      for k, v in var.managed_folders :
      v.parent == null || try(var.managed_folders[v.parent].parent, null) == null
    ])
    error_message = "managed_folders supports one level of nesting: a parent cannot itself have a parent."
  }
}

variable "grants" {
  description = <<-DESC
    Permissions to grant, keyed by a name of your choosing. folder refers to a
    key in existing_folders or managed_folders.

    One grant per role: to give two groups different roles on one folder,
    declare two grants.
  DESC

  type = map(object({
    folder    = string
    role      = optional(string, "VIEWER")
    group_ids = optional(list(string))
    user_ids  = optional(list(string))
  }))

  default = {}
}
