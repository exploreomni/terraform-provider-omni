variable "models" {
  description = <<-DESC
    Models to create, keyed by name. Set git to connect the model to a
    repository; leave it null for a model edited in Omni.
  DESC

  type = map(object({
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
