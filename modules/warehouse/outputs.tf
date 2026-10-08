output "connection_ids" {
  value = { for k, c in omni_connection.this : k => c.id }
}
