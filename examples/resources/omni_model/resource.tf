data "omni_model" "shared" {
  name       = "Production Snowflake"
  model_kind = "SHARED"
}

resource "omni_model" "finance_extension" {
  name          = "finance_metrics"
  model_kind    = "SHARED_EXTENSION"
  base_model_id = data.omni_model.shared.id
  connection_id = data.omni_model.shared.connection_id
}
