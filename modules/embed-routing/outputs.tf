output "environment_ids" {
  description = "Tenant key to connection environment ID."
  value       = { for k, e in omni_connection_environment.tenant : k => e.id }
}

output "routed_tenants" {
  description = "Tenants with a connection of their own. Everyone else uses the base connection."
  value       = keys(var.routes)
}
