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

  # Both checks resolve to coalesce(v.parent, k), never to v.parent directly.
  # Terraform's || does not short-circuit, so a guard of the form
  # "v.parent == null || f(v.parent)" still calls f with null and fails on the
  # argument rather than reporting the condition. Substituting the folder's own
  # key when it has no parent makes the check trivially true for a top-level
  # folder and passes a real key in every case.
  validation {
    condition = alltrue([
      for k, v in var.managed_folders :
      contains(keys(var.managed_folders), coalesce(v.parent, k))
    ])
    error_message = "Every parent must be another key in managed_folders."
  }

  validation {
    # One level of nesting. A child of a child would otherwise fail later with
    # an unhelpful index error, so it is caught here instead.
    condition = alltrue([
      for k, v in var.managed_folders :
      try(var.managed_folders[coalesce(v.parent, k)].parent, null) == null
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
