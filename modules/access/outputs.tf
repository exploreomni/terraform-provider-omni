output "user_ids" {
  value = { for k, u in omni_user.this : k => u.id }
}

output "group_ids" {
  value = { for k, g in omni_user_group.this : k => g.id }
}

output "connection_role_ids" {
  description = "Groups holding a connection-scoped role."
  value       = { for k, r in omni_user_group_model_role.connection : k => r.id }
}
