variable "users" {
  description = "Internal users, keyed by email. Requires an organization API key."
  type = map(object({
    display_name = string
    attributes   = optional(map(string))
  }))
  default = {}
}

variable "groups" {
  description = "Internal groups, keyed by display name."
  type = map(object({
    # Overrides the key as the visible name. Set this to rename a group without
    # re-keying it, which would replace it instead.
    display_name = optional(string)

    members = optional(list(string), [])

    # Whether this group gets a role on a model. Set from configuration rather
    # than inferred from model_id, which is an apply-time value and so cannot
    # decide which groups the role resource covers.
    grant_model_role = optional(bool, false)

    model_id      = optional(string)
    connection_id = optional(string)
    model_role    = optional(string, "QUERIER")

    # A role on the whole connection rather than one model. Usually
    # CONNECTION_ADMIN. Requires connection_id and ignores model_id.
    connection_role = optional(string)
  }))
  default = {}
}

variable "folders" {
  description = "Internal folders, keyed by path."
  type = map(object({
    name     = string
    scope    = optional(string)
    owner_id = optional(string)
    groups   = optional(list(string), [])
    role     = optional(string, "EDITOR")
  }))
  default = {}

  validation {
    condition     = alltrue([for k, v in var.folders : v.scope != null])
    error_message = "Every folder must state a scope: organization or restricted. Left unset, the provider silently chooses organization, which is a decision about who can reach the folder."
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      contains(["organization", "restricted"], coalesce(v.scope, "organization"))
    ])
    error_message = "scope must be organization or restricted."
  }

  validation {
    condition = alltrue([
      for k, v in var.folders :
      coalesce(v.scope, "organization") != "restricted" || v.owner_id != null
    ])
    error_message = "A restricted folder needs owner_id when the provider uses an organization API key."
  }
}
