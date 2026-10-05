output "connection_ids" {
  value = { for k, c in omni_connection.this : k => c.id }
}

output "schedule_ids" {
  value = { for k, s in omni_connection_schedule.this : k => s.id }
}
