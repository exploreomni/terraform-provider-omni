resource "omni_model_git" "sales_ssh" {
  model_id  = omni_model.sales.id
  clone_url = "git@github.com:acme/omni-models.git"

  auth_method = "ssh"

  # Never returned by the API, so Terraform cannot detect drift on it. Keep it
  # in a variable or a secret store rather than in the configuration.
  deploy_private_key = var.omni_model_deploy_key

  base_branch = "main"
  model_path  = "models/sales"
  web_url     = "https://github.com/acme/omni-models"

  # Model changes go through a pull request rather than straight to the branch.
  require_pull_request = "always"
}

resource "omni_model_git" "sales_staging" {
  model_id  = omni_model.sales_staging.id
  clone_url = "https://github.com/acme/omni-models.git"

  auth_method = "https_token"
  token       = var.omni_model_git_token

  # The model mirrors the repository and cannot be edited in Omni. Use this
  # for an environment that follows another rather than being authored in.
  git_follower = true
}
