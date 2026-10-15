variable "attributes" {
  description = <<-DESC
    Attribute definitions, keyed by a local name the rest of the configuration
    references.

    existing_id references a definition made elsewhere instead of creating one.
    Without it the definition is created and label is required; name defaults to
    the key.

    The root module validates the combinations and names the entry at fault;
    these are the same checks, kept so the module stands on its own.
  DESC

  type = map(object({
    # Reference a definition made elsewhere rather than creating one.
    existing_id = optional(string)

    label           = optional(string)
    name            = optional(string)
    type            = optional(string, "String")
    description     = optional(string)
    multiple_values = optional(bool, false)
    default_value   = optional(string)
    default_values  = optional(list(string))
  }))

  default = {}

  validation {
    condition = alltrue([
      for k, v in var.attributes : v.existing_id != null || v.label != null
    ])
    error_message = "Every attribute needs existing_id to reference one, or label to create one."
  }

  validation {
    condition = alltrue([
      for k, v in var.attributes :
      v.existing_id == null || (v.label == null && v.name == null && v.description == null && v.default_value == null && v.default_values == null)
    ])
    error_message = "A referenced attribute already has its label, name and default; Terraform does not change them."
  }

  validation {
    condition = alltrue([
      for k, v in var.attributes : v.default_value == null || v.default_values == null
    ])
    error_message = "Set default_value for one default or default_values for several, not both."
  }

  validation {
    condition = alltrue([
      for k, v in var.attributes : v.default_values == null || v.multiple_values
    ])
    error_message = "default_values needs multiple_values = true; a single-valued attribute holds one default."
  }
}

variable "assignments" {
  description = <<-DESC
    Values per person, as an email to a map of keys from var.attributes.

    Per person because Omni has no group-assigned value: a user's value comes
    from a direct assignment or the definition's default, and nothing else. The
    group-scoped control that does exist is var.selectable_values, which is a
    different thing.
  DESC

  type    = map(map(string))
  default = {}

  validation {
    condition = alltrue(flatten([
      for email, assigned in var.assignments : [
        for key in keys(assigned) : contains(keys(var.attributes), key)
      ]
    ]))
    error_message = format(
      "These assignments use an attribute key that is not in var.attributes: %s.",
      join("; ", flatten([
        for email, assigned in var.assignments : [
          for key in keys(assigned) : "${email} uses \"${key}\""
          if !contains(keys(var.attributes), key)
        ]
      ]))
    )
  }
}
