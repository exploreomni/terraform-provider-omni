# Schema refresh schedules.
#
# A separate module from warehouse, for an ordering reason rather than a
# conceptual one.
#
# A schedule cannot be created until the connection has a schema model. Until
# then POST /v1/connections/{id}/schedules answers "Connection with id ... does
# not exist", even though GET and PATCH on the connection return 200, so the
# error names the wrong object.
#
# Schema models are created by the modeling module, which depends on warehouse
# because a model is built on a connection. If the schedule lived in warehouse
# it would be created before the schema model existed, and expressing the real
# order would need warehouse to depend on modeling, which is a cycle.
#
# Splitting the schedule out breaks it: warehouse creates connections, modeling
# creates the schema models, and this module runs after both.

terraform {
  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

resource "omni_connection_schedule" "this" {
  for_each = var.schedules

  connection_id = each.value.connection_id

  # Six-field EventBridge cron: minute hour day-of-month month day-of-week year.
  # Daily at 02:00 is "0 2 * * ? *".
  schedule     = each.value.cron
  timezone     = each.value.timezone
  hard_refresh = each.value.hard_refresh
}
