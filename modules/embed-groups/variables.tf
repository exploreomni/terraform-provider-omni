variable "tenants" {
  description = <<-DESC
    Groups to create, keyed by tenant. The key becomes the group name with the
    prefix applied, and is what a signed URL passes in "groups".

    Set model_id and connection_id to also grant a role on a model. Omit them
    and only the group is created.
  DESC

  type = map(object({
    model_id      = optional(string)
    connection_id = optional(string)
    model_role    = optional(string, "QUERY_TOPICS")
  }))

  default = {}
}

variable "prefix" {
  description = "Prefix for group names. Keeps tenant groups recognisable alongside internal ones."
  type        = string
  default     = "tenant-"
}
