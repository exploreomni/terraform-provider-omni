variable "folders" {
  description = <<-DESC
    Folders, keyed by a local name the grants reference.

    existing_path looks a folder up by its path instead of creating it. Without
    it the folder is created and name is required; parent nests it one level
    inside another key, and a nested folder inherits its parent's scope.

    The root module validates the combinations and names the folder at fault;
    these are the same checks, kept here so the module stands on its own.
  DESC

  type = map(object({
    existing_path      = optional(string)
    name               = optional(string)
    parent             = optional(string)
    scope              = optional(string)
    owner_id           = optional(string)
    delete_recursively = optional(bool, false)
  }))

  default = {}

  validation {
    condition = alltrue([
      for k, v in var.folders : v.existing_path != null || v.name != null
    ])
    error_message = "Every folder needs existing_path to look one up, or name to create one."
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      v.existing_path != null || v.parent != null || v.scope != null
    ])
    error_message = "A created top-level folder must state a scope: organization or restricted."
  }

  validation {
    condition = alltrue([
      for k, v in var.folders : v.parent == null || v.scope == null
    ])
    error_message = "A nested folder inherits its parent's scope and must not state one."
  }
}

variable "grants" {
  description = <<-DESC
    Permissions to grant, keyed by a name of your choosing. folder refers to a
    key in var.folders.

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
