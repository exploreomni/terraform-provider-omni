output "group_ids" {
  description = "Tenant key to group ID."
  value       = { for k, g in omni_user_group.tenant : k => g.id }
}

output "group_names" {
  description = "Tenant key to the name a signed URL passes in groups."
  value       = { for k, g in omni_user_group.tenant : k => g.display_name }
}
