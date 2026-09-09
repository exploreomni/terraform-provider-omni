resource "omni_model_yaml_file" "orders_topic" {
  model_id  = omni_model.finance_extension.id
  file_name = "orders.topic"
  mode      = "extension"

  # Keep the YAML in the repo so review happens in git, not in the UI.
  yaml = file("${path.module}/model/orders.topic.yaml")

  commit_message = "terraform: sync orders topic"
}
