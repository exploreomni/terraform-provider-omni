# End to end: a connection, an extension model on top of it, the YAML that
# defines a topic, a folder for the output, and the people who can use it.

terraform {
  required_providers {
    omni = {
      source  = "ashima-omni/omni"
      version = "~> 0.1"
    }
  }
}

provider "omni" {}

resource "omni_connection" "warehouse" {
  name      = "Production Snowflake"
  dialect   = "snowflake"
  host      = var.snowflake_account
  database  = "ANALYTICS"
  warehouse = "COMPUTE_WH"
  username  = "OMNI_SVC"
  password  = var.snowflake_password
  base_role = "NO_ACCESS"
}

resource "omni_model" "finance" {
  name          = "finance_metrics"
  model_kind    = "SHARED_EXTENSION"
  connection_id = omni_connection.warehouse.id
}

resource "omni_model_yaml_file" "orders_topic" {
  model_id       = omni_model.finance.id
  file_name      = "orders.topic"
  mode           = "extension"
  yaml           = file("${path.module}/orders.topic.yaml")
  commit_message = "terraform: manage orders topic"
}

resource "omni_folder" "finance" {
  name = "Finance"
  path = "finance"
}

resource "omni_user_group" "finance" {
  display_name = "Finance"
  member_ids   = [for u in omni_user.analysts : u.id]
}

resource "omni_user" "analysts" {
  for_each = toset(var.analyst_emails)

  user_name    = each.value
  display_name = each.value
}

resource "omni_user_group_model_role" "finance_querier" {
  user_group_id = omni_user_group.finance.id
  model_id      = omni_model.finance.id
  role_name     = "QUERIER"
}

variable "snowflake_account" {
  type = string
}

variable "snowflake_password" {
  type      = string
  sensitive = true
}

variable "analyst_emails" {
  type    = list(string)
  default = []
}
