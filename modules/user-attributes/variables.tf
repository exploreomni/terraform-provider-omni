variable "attributes" {
  description = <<-DESC
    Attribute definitions to reference, as a local key to the definition's ID.
    Find an ID with:

      curl -s -H "Authorization: Bearer $OMNI_API_TOKEN" \
        "$OMNI_BASE_URL/api/v1/user-attributes"

    IDs rather than names on purpose. A definition can be renamed in the UI, and
    a configuration naming it would stop matching silently.

    The local key is yours and is what var.assignments refers to, so renaming an
    attribute in Omni changes nothing here.
  DESC

  type    = map(string)
  default = {}
}

variable "assignments" {
  description = <<-DESC
    Attribute values per person, as an email to a map of local key to value.

      assignments = {
        "analyst@acme.com" = {
          tenant = "acme"
          region = "AMER"
        }
      }

    Every key used here must appear in var.attributes.

    Values are per person. Omni has no API for assigning them to a group: the
    SCIM group resource has no attribute extension and the only attribute
    endpoint is a read.
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
