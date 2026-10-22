resource "omni_connection_schedule" "nightly" {
  connection_id = omni_connection.warehouse.id

  # Six fields, not five: minute hour day-of-month month day-of-week year.
  # This is AWS EventBridge cron, so day-of-week and day-of-month cannot both
  # be "*" - one of them has to be "?". Daily at 02:00:
  schedule = "0 2 * * ? *"
  timezone = "Australia/Melbourne"
}

resource "omni_connection_schedule" "weekly_rebuild" {
  connection_id = omni_connection.warehouse.id

  # Sundays at 03:30.
  schedule = "30 3 ? * SUN *"
  timezone = "Australia/Melbourne"

  # Discards the schema model and rebuilds it, rather than merging in newly
  # generated views. This drops model customisations, so turn it on only where
  # the model is generated and nothing has been hand-edited.
  hard_refresh = true
}
