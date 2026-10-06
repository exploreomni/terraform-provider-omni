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
    name   = string
    scope  = optional(string, "organization")
    groups = optional(list(string), [])
    role   = optional(string, "EDITOR")
  }))
  default = {}
}
