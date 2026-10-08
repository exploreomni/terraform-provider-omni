variable "models" {
  description = <<-DESC
    Models to create, keyed by a stable identifier. Set git to connect the
    model to a repository; leave it null for a model edited in Omni.

    The key is the model's identity and is what other modules reference. name
    overrides it as the visible model name. Set name to rename a model: the API
    renames in place, while re-keying destroys the model and creates a new one,
    which loses its content.

    kind = "SCHEMA" creates the connection's schema model, the equivalent of
    the UI's "Build Schema". A connection needs one before anything else can be
    built on it, and schema models are created ahead of every other kind.

    kind, connection_id and base_model_id all force replacement.
  DESC

  type = map(object({
    # Overrides the key as the model's name in Omni.
    name = optional(string)

    # Start a schema refresh once the model is created. Left unset, SCHEMA
    # models refresh and others do not, which is the behaviour you want: a
    # schema model that has not been refreshed has no tables.
    refresh_on_create = optional(bool)

    kind          = optional(string, "SHARED_EXTENSION")
    connection_id = optional(string)
    base_model_id = optional(string)

    git = optional(object({
      clone_url                  = string
      auth_method                = optional(string, "ssh")
      base_branch                = optional(string, "main")
      branch_per_pull_request    = optional(bool, false)
      require_pull_request       = optional(string, "always")
      follower                   = optional(bool, false)
      service_provider           = optional(string, "auto")
      model_path                 = optional(string)
      github_app_installation_id = optional(string)
    }))
  }))

  default = {}
}

variable "git_credentials" {
  description = "Git credentials per model, keyed the same as models. Pass from secrets."
  sensitive   = true

  type = map(object({
    deploy_private_key    = optional(string)
    deploy_key_passphrase = optional(string)
    token                 = optional(string)
  }))

  default = {}
}
