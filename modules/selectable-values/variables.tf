variable "provisions" {
  description = <<-DESC
    Selectable values, keyed by a local name. IDs rather than references: the
    root resolves the attribute, group and user keys, because this module runs
    after the ones that own them.

    Exactly one of group_id or user_id. The provider enforces that; the root
    checks it first so the message can name the entry.
  DESC

  type = map(object({
    user_attribute_id = string

    group_id = optional(string)

    # The membership ID, which is not the user ID. The API does not say which
    # one it received, so a wrong value here provisions nothing and reports
    # success.
    user_id = optional(string)

    values          = optional(list(string))
    allow_any_value = optional(bool, false)
  }))

  default = {}
}
