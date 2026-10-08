variable "users" {
  description = "Internal users, keyed by email. Requires an organization API key."
  type = map(object({
    display_name = string
    attributes   = optional(map(string))
  }))
  default = {}
}

variable "groups" {
  description = "Groups, keyed by a stable name. One kind of group; see manage_members."
  type = map(object({
    # Overrides the key as the visible name. Set this to rename a group without
    # re-keying it, which would replace it instead.
    display_name = optional(string)

    # Prepended to the key when display_name is unset. Keeps a family of groups
    # recognisable without putting the prefix in every key.
    #
    # Defaults to "" rather than null deliberately: the display_name expression
    # interpolates this, and coalesce rejects empty strings as well as nulls, so
    # a null here would make coalesce(name_prefix, "") fail outright.
    name_prefix = optional(string, "")

    members = optional(list(string), [])

    # Whether Terraform owns the membership list.
    #
    # true, the default: members below are authoritative, and anyone added
    # outside Terraform is removed on the next apply.
    #
    # false: member_ids is left unset and Omni keeps whatever membership it has.
    # This is what an embed group needs, because its sessions create their own
    # users and an authoritative list would delete them.
    manage_members = optional(bool, true)

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
