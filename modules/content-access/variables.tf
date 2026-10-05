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
