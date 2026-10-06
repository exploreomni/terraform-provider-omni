variable "tenants" {
  description = <<-DESC
    Groups to create, keyed by tenant. The key becomes the group name with the
    prefix applied, and is what a signed URL passes in "groups".

    Set grant_model_role to also grant a role on a model, and supply model_id
    and connection_id with it. Leave it false and only the group is created.

    grant_model_role is separate from model_id because it has to be known at
    plan time: model_id is usually an attribute of a model created in the same
    run, and Terraform cannot decide which groups get a role from a value it
    does not have yet.
  DESC

  type = map(object({
    grant_model_role = optional(bool, false)
    model_id         = optional(string)
    connection_id    = optional(string)
    model_role       = optional(string, "QUERY_TOPICS")
  }))

  # No validation on model_id or connection_id here: both are apply-time values
  # in normal use, and a check on an unknown cannot say anything useful. The
  # root module validates the configuration they are derived from instead.
  default = {}
}

variable "prefix" {
  description = "Prefix for group names. Keeps tenant groups recognisable alongside internal ones."
  type        = string
  default     = "tenant-"
}
