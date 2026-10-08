output "schedule_ids" {
  description = "Connection key to schedule ID."
  value       = { for k, s in omni_connection_schedule.this : k => s.id }
}
