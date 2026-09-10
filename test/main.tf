# End-to-end CRUD exercise for every resource and data source in the provider.
#
# Phases are driven by variables, not edits:
#   create  terraform apply
#   update  terraform apply -var suffix=v2 -var role_name=VIEWER ...
#   delete  terraform destroy
#
# Run it against a playground instance. It creates real users, groups, models
# and folders.

terraform {
  required_version = ">= 1.5"

  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

provider "omni" {}

# --------------------------------------------------------------------------
# Data sources: look up things that already exist.
# --------------------------------------------------------------------------

data "omni_connection" "existing" {
  name = var.connection_name
}

data "omni_model" "base" {
  name       = var.base_model_name
  model_kind = "SHARED"
}

# --------------------------------------------------------------------------
# Folders: parent, child, and a deliberately deep nesting check.
# --------------------------------------------------------------------------

resource "omni_folder" "parent" {
  name = "tf-test-${var.suffix}"
  path = "tf-test-${var.suffix}"
}

resource "omni_folder" "child" {
  name             = "tf-test-child-${var.suffix}"
  parent_folder_id = omni_folder.parent.id

  # The parent is not empty at destroy time, so it needs the recursive flag.
  delete_recursively = true
}

# --------------------------------------------------------------------------
# Users and groups.
# --------------------------------------------------------------------------

resource "omni_user" "test" {
  user_name    = var.test_user_email
  display_name = "TF Provider Test ${var.suffix}"

  attributes = var.user_attribute_key == "" ? null : {
    (var.user_attribute_key) = var.suffix
  }
}

resource "omni_user_group" "test" {
  display_name = "tf-test-group-${var.suffix}"
  member_ids   = [omni_user.test.id]
}

data "omni_user" "by_email" {
  user_name  = var.test_user_email
  depends_on = [omni_user.test]
}

data "omni_user_group" "by_name" {
  display_name = "tf-test-group-${var.suffix}"
  depends_on   = [omni_user_group.test]
}

# --------------------------------------------------------------------------
# Model and model YAML.
# --------------------------------------------------------------------------

resource "omni_model" "extension" {
  name          = "tf_test_ext_${var.suffix}"
  model_kind    = "SHARED_EXTENSION"
  connection_id = data.omni_connection.existing.id
  base_model_id = data.omni_model.base.id
}

resource "omni_model_yaml_file" "topic" {
  model_id  = omni_model.extension.id
  file_name = "tf_test.topic"
  mode      = "extension"

  yaml = <<-YAML
    # Managed by the Terraform provider test suite.
    label: TF Test Topic
    description: ${var.topic_description}
  YAML
}

# --------------------------------------------------------------------------
# Role assignments. There is no delete endpoint, so destroy downgrades these
# to role_on_destroy instead of removing them.
# --------------------------------------------------------------------------

resource "omni_user_model_role" "test" {
  user_id       = omni_user.test.id
  model_id      = omni_model.extension.id
  connection_id = data.omni_connection.existing.id
  role_name     = var.role_name
}

resource "omni_user_group_model_role" "test" {
  user_group_id = omni_user_group.test.id
  model_id      = omni_model.extension.id
  connection_id = data.omni_connection.existing.id
  role_name     = var.role_name
}

# --------------------------------------------------------------------------
# Connection: opt in, since it needs real warehouse credentials. base_role and
# password update in place; everything else forces replacement.
# --------------------------------------------------------------------------

resource "omni_connection" "test" {
  count = var.test_connection ? 1 : 0

  name     = "tf-test-connection-${var.suffix}"
  dialect  = var.connection_dialect
  host     = var.connection_host
  database = var.connection_database
  username = var.connection_username
  password = var.connection_password

  base_role = var.connection_base_role
}
