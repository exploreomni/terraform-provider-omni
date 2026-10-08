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
    scope              = optional(string)
    owner_id           = optional(string)
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

  validation {
    condition = alltrue([
      for k, v in var.managed_folders :
      contains(["organization", "restricted"], coalesce(v.scope, "organization"))
    ])
    error_message = "scope must be organization or restricted."
  }

  validation {
    # A top-level folder states its scope. Left unset, the provider silently
    # chooses organization, which is the behaviour this module exists to make
    # visible.
    condition = alltrue([
      for k, v in var.managed_folders :
      v.parent != null || v.scope != null
    ])
    error_message = "Every top-level folder in managed_folders must state a scope: organization or restricted."
  }

  validation {
    # Scope is inherited, so a child setting one can only disagree with its parent.
    condition = alltrue([
      for k, v in var.managed_folders :
      v.parent == null || v.scope == null
    ])
    error_message = "A nested folder inherits its parent's scope and must not set scope itself."
  }

  validation {
    condition = alltrue([
      for k, v in var.managed_folders :
      coalesce(v.scope, "organization") != "restricted" || v.owner_id != null
    ])
    error_message = "A restricted folder needs owner_id when the provider uses an organization API key."
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

  validation {
    # The API rejects a permission with neither, and its message
    # ("userIds or userGroupIds must be provided") does not say which grant.
    #
    # group_ids is resolved from names at apply time, so this cannot tell "none
    # configured" from "not created yet". The root module checks the names,
    # which are known at plan time; this is the backstop for direct callers.
    condition = alltrue([
      for k, v in var.grants :
      length(coalesce(v.group_ids, [])) + length(coalesce(v.user_ids, [])) > 0
    ])
    error_message = "A grant must name at least one group or user. Remove the grant rather than leaving it empty."
  }
}

variable "document_grants" {
  description = <<-DESC
    Grants on individual documents, keyed by local name. group_ids and user_ids
    are resolved by the caller; document_id is a literal identifier, because
    documents are made in the UI and there is no name to resolve.
  DESC

  type = map(object({
    document_id  = string
    role         = optional(string, "VIEWER")
    group_ids    = optional(list(string), [])
    user_ids     = optional(list(string))
    access_boost = optional(bool)
  }))

  default = {}
}
