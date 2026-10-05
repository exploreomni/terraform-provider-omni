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
    members       = optional(list(string), [])
    model_id      = optional(string)
    connection_id = optional(string)
    model_role    = optional(string, "QUERIER")
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
