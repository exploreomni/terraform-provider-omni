output "model_ids" {
  value = { for k, m in omni_model.this : k => m.id }
}
