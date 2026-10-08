output "model_ids" {
  description = "Model key to model ID, for schema models and the models built on them alike."
  value       = local.model_ids
}

output "schema_model_ids" {
  description = <<-DESC
    Schema model key to ID.

    The provider starts a refresh when it creates a SCHEMA model, so this is
    normally only needed to refresh again by hand:

      curl -X POST -H "Authorization: Bearer $OMNI_API_TOKEN" \
        "$OMNI_BASE_URL/api/v1/models/<id>/refresh"
  DESC
  value       = { for k, m in omni_model.schema : k => m.id }
}
